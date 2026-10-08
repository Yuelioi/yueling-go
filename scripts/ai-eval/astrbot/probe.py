"""Exercise upstream AstrBot runner, OpenAI provider and local tool executor unchanged."""
import asyncio
import json
import os
import sys
import subprocess
import time
from pathlib import Path
from types import SimpleNamespace

ROOT = Path(os.environ.get('ASTRBOT_EVAL_OUTPUT', '/private/tmp/yueling-eval-astrbot-20260922'))
ROOT.mkdir(parents=True, exist_ok=True)
os.environ['ASTRBOT_ROOT'] = str(ROOT / 'runtime')
SOURCE = Path(os.environ.get('ASTRBOT_SOURCE', '/private/tmp/yueling-research-astrbot-20260922'))
REVISION = '95e98b8aed75d56713666eff39e31bafffd95426'
actual_revision = subprocess.check_output(['git', '-C', str(SOURCE), 'rev-parse', 'HEAD'], text=True).strip()
if actual_revision != REVISION:
    raise SystemExit('AstrBot source revision mismatch: '+actual_revision)
sys.path.insert(0, str(SOURCE))
from aiohttp import web
from astrbot.core.agent.hooks import BaseAgentRunHooks
from astrbot.core.agent.message import dump_messages_with_checkpoints
from astrbot.core.agent.run_context import ContextWrapper
from astrbot.core.agent.runners.tool_loop_agent_runner import ToolLoopAgentRunner
from astrbot.core.agent.tool import FunctionTool, ToolSet
from astrbot.core.astr_agent_tool_exec import FunctionToolExecutor
from astrbot.core.provider.entities import ProviderRequest
from astrbot.core.provider.sources.openai_source import ProviderOpenAIOfficial


class Event:
    def get_extra(self, key, default=None):
        return default
    def get_result(self):
        return None


def completion(*, text=None, tool=False, reasoning=None, call_id='call_1'):
    msg = {'role': 'assistant', 'content': text}
    if reasoning is not None:
        msg['reasoning_content'] = reasoning
    if tool:
        msg['tool_calls'] = [{'id': call_id, 'type': 'function', 'function': {'name': tool if isinstance(tool, str) else 'summarize_chat', 'arguments': '{}'}}]
    return {'id': 'chatcmpl-probe', 'object': 'chat.completion', 'created': 1, 'model': 'fixture-model', 'choices': [{'index': 0, 'message': msg, 'finish_reason': 'tool_calls' if tool else 'stop'}], 'usage': {'prompt_tokens': 20, 'completion_tokens': 10, 'total_tokens': 30}}


class ModelServer:
    def __init__(self, script):
        self.script = list(script)
        self.requests = []
        self.started = asyncio.Event()
    async def handle(self, request):
        body = await request.json()
        self.requests.append(body)
        self.started.set()
        if not self.script:
            return web.json_response({'error': {'message': 'fixture script exhausted', 'type': 'invalid_request_error'}}, status=400)
        item = self.script.pop(0)
        if item == 'hang':
            await asyncio.sleep(30)
            return web.json_response(completion(text='late'))
        if isinstance(item, int):
            return web.json_response({'error': {'message': 'secret-provider-body', 'type': 'api_error'}}, status=item)
        if body.get('stream'):
            response = web.StreamResponse(headers={'Content-Type': 'text/event-stream'})
            await response.prepare(request)
            msg = item['choices'][0]['message']
            for key in ('reasoning_content', 'content', 'tool_calls'):
                if msg.get(key) is None:
                    continue
                delta = {key: msg[key]}
                if key == 'tool_calls':
                    delta[key] = [dict(t, index=i) for i,t in enumerate(msg[key])]
                chunk = {'id': 'chunk-probe', 'object': 'chat.completion.chunk', 'created': 1, 'model': 'fixture-model', 'choices': [{'index': 0, 'delta': delta, 'finish_reason': None}]}
                await response.write(('data: '+json.dumps(chunk)+'\n\n').encode())
            chunk = {'id': 'chunk-probe', 'object': 'chat.completion.chunk', 'created': 1, 'model': 'fixture-model', 'choices': [{'index': 0, 'delta': {}, 'finish_reason': item['choices'][0]['finish_reason']}], 'usage': item['usage']}
            await response.write(('data: '+json.dumps(chunk)+'\n\ndata: [DONE]\n\n').encode())
            await response.write_eof()
            return response
        return web.json_response(item)
    async def __aenter__(self):
        app = web.Application()
        app.router.add_post('/v1/chat/completions', self.handle)
        self.server = web.AppRunner(app, shutdown_timeout=0.1)
        await self.server.setup()
        site = web.TCPSite(self.server, '127.0.0.1', 0)
        await site.start()
        self.url = 'http://127.0.0.1:'+str(site._server.sockets[0].getsockname()[1])+'/v1'
        return self
    async def __aexit__(self, *args):
        await self.server.cleanup()


async def new_runner(server, *, contexts=None, session='g100-u10', streaming=False, tool_delay=0, timeout=3, prompt='总结群聊'):
    stats = {'tool_calls': 0, 'tool_cancelled': False, 'reminder_calls': 0}
    async def read_history(event):
        stats['tool_calls'] += 1
        try:
            if tool_delay:
                await asyncio.sleep(tool_delay)
        except asyncio.CancelledError:
            stats['tool_cancelled'] = True
            raise
        return '甲：周五发布；乙：周四测试。'
    async def create_reminder(event):
        stats['tool_calls'] += 1
        stats['reminder_calls'] += 1
        return 'receipt=fixture-1'
    reminder = FunctionTool(name='create_reminder', description='Create a fixture reminder only', parameters={'type':'object','properties':{}}, handler=create_reminder)
    tool = FunctionTool(name='summarize_chat', description='Read fictional chat messages', parameters={'type':'object','properties':{}}, handler=read_history)
    provider = ProviderOpenAIOfficial({'id':'fixture','type':'openai_chat_completion','key':['fixture-not-a-secret'],'api_base':server.url,'model':'fixture-model','timeout':timeout}, {})
    runner = ToolLoopAgentRunner()
    ctx = ContextWrapper(context=SimpleNamespace(event=Event()), tool_call_timeout=1)
    await runner.reset(provider=provider, request=ProviderRequest(prompt=prompt, contexts=contexts or [], session_id=session, func_tool=ToolSet([tool, reminder])), run_context=ctx, tool_executor=FunctionToolExecutor(), agent_hooks=BaseAgentRunHooks(), streaming=streaming, request_max_retries=2)
    return runner, provider, stats


async def consume(runner):
    events=[]
    async for event in runner.step_until_done(max_step=5):
        chain = (event.data or {}).get('chain')
        events.append({'event_type':event.type, 'chain_type':getattr(chain,'type',None), 'text':chain.get_plain_text() if chain else ''})
    return events


async def main():
    results={}
    async with ModelServer([completion(tool=True, reasoning='reasoning-only'), completion(text='周五发布，周四测试。')]) as server:
        runner, provider, stats = await new_runner(server, streaming=True)
        events = await consume(runner)
        assert stats['tool_calls'] == 1
        assert any(e['chain_type']=='reasoning' and 'reasoning-only' in e['text'] for e in events)
        assert any(e['chain_type']=='tool_call' for e in events)
        assert all('reasoning-only' not in e['text'] for e in events if e['chain_type'] not in ('reasoning',))
        history=dump_messages_with_checkpoints(runner.run_context.messages)
        results['stream_event_separation']={'pass':True,'events':events,'http_requests':len(server.requests),'tool_calls':stats['tool_calls'],'side_effect_calls':stats['reminder_calls']}
        await provider.client.close()
    async with ModelServer([completion(tool='create_reminder'),503,completion(text='恢复后：周五发布，周四测试。')]) as server:
        runner,provider,stats=await new_runner(server)
        events=await consume(runner)
        assert stats['tool_calls']==1 and len(server.requests)==3
        assert server.requests[1]['messages']==server.requests[2]['messages']
        results['503_after_tool']={'pass':True,'http_requests':len(server.requests),'tool_calls':stats['tool_calls'],'side_effect_calls':stats['reminder_calls'],'retry_same_history':True,'events':events}
        await provider.client.close()
    async with ModelServer([completion(tool='create_reminder'),503,503]) as server:
        runner,provider,stats=await new_runner(server)
        events=await consume(runner)
        assert stats['tool_calls']==1
        results['permanent_503_after_tool']={'tool_calls':stats['tool_calls'],'side_effect_calls':stats['reminder_calls'],'http_requests':len(server.requests),'state':str(runner.state),'exposes_provider_detail':any('secret-provider-body' in e['text'] for e in events),'events':events}
        await provider.client.close()
    async with ModelServer([completion(tool='create_reminder'),completion(tool='create_reminder',call_id='call_2'),completion(text='完成')]) as server:
        runner,provider,stats=await new_runner(server)
        events=await consume(runner)
        results['model_repeats_identical_tool']={'deduplicated':stats['tool_calls']==1,'tool_calls':stats['tool_calls'],'side_effect_calls':stats['reminder_calls'],'events':events}
        await provider.client.close()
    async with ModelServer([completion(text='周四测试。'),completion(text='没有群100的信息。')]) as server:
        runner,provider,stats=await new_runner(server,contexts=history,prompt='周几测试？')
        events=await consume(runner)
        await provider.client.close()
        runner2,provider2,stats2=await new_runner(server,session='g999-u10')
        await consume(runner2)
        first=json.dumps(server.requests[0]['messages'],ensure_ascii=False)
        second=json.dumps(server.requests[1]['messages'],ensure_ascii=False)
        assert '甲：周五发布' in first and '甲：周五发布' not in second
        results['caller_supplied_history_isolation']={'pass':True,'history_reused_first':True,'other_runner_no_history':True,'persistent_session_store_tested':False}
        await provider2.client.close()
    async with ModelServer([completion(text='<tool_call>summarize_chat({})</tool_call>')]) as server:
        runner,provider,stats=await new_runner(server)
        events=await consume(runner)
        results['literal_tool_markup']={'suppressed':not any('<tool_call>' in e['text'] for e in events if e['chain_type'] in (None,'normal')), 'events':events}
        await provider.client.close()
    async with ModelServer(['hang']) as server:
        runner,provider,stats=await new_runner(server)
        task=asyncio.create_task(consume(runner))
        await asyncio.wait_for(server.started.wait(),2)
        start=time.monotonic()
        runner.request_stop()
        events=await asyncio.wait_for(task,2)
        results['cancel_waiting_http']={'pass':runner.was_aborted(),'elapsed_seconds':round(time.monotonic()-start,3),'events':events,'http_requests':len(server.requests)}
        await provider.client.close()
    async with ModelServer([completion(tool=True),completion(text='工具已超时')]) as server:
        runner,provider,stats=await new_runner(server,tool_delay=30)
        start=time.monotonic()
        events=await asyncio.wait_for(consume(runner),5)
        assert stats['tool_cancelled'] and stats['tool_calls']==1
        results['tool_timeout']={'pass':True,'elapsed_seconds':round(time.monotonic()-start,3),'tool_cancelled':True,'tool_calls':stats['tool_calls'],'side_effect_calls':stats['reminder_calls'],'events':events}
        await provider.client.close()
    results['scope'] = {'level': 'component', 'production_provider_and_runner': True, 'production_local_tool_executor': True, 'full_openapi_pipeline': False, 'real_model': False, 'real_qq': False, 'source_revision': actual_revision}
    (ROOT/'results.json').write_text(json.dumps(results,ensure_ascii=False,indent=2))
    print(json.dumps({k:{n:v for n,v in r.items() if n!='events'} for k,r in results.items()},ensure_ascii=False,indent=2))

if __name__=='__main__':
    asyncio.run(main())
