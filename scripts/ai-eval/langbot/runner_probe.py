"""Real unmodified LocalAgentRunner, ResponseWrapper and SessionManager probes.

Application boot is replaced only to avoid importing every platform/database driver.
An HTTPX I/O fixture implements the provider boundary against localhost;
this does NOT validate LangBot's LiteLLMRequester or complete pipeline boot.
"""
import asyncio
import json
import logging
import os
import sys
import types
from pathlib import Path
from types import SimpleNamespace as NS
from unittest.mock import AsyncMock

# These modules reference Application for type annotations; no production method
# under test is replaced. The actual fixture Application is injected below.
boot_stub=types.ModuleType('langbot.pkg.core.app')
sys.modules['langbot.pkg.core.app']=boot_stub

import aiohttp.web
import httpx
from langbot.pkg.api.http.context import ExecutionContext
from langbot.pkg.provider.runners.localagent import LocalAgentRunner
from langbot.pkg.provider.session.sessionmgr import SessionManager
from langbot.pkg.pipeline.wrapper.wrapper import ResponseWrapper
from langbot.pkg.platform.sources.http_bot import HttpBotAdapter
from langbot_plugin.api.entities.builtin.pipeline.query import Query
from langbot_plugin.api.entities.builtin.provider.message import Message, ToolCall, FunctionCall
from langbot_plugin.api.entities.builtin.provider.session import LauncherTypes

OUT=Path(os.environ.get('AI_EVAL_OUTPUT',str(Path(__file__).parent)));OUT.mkdir(parents=True,exist_ok=True)
report={'scope':'real LocalAgentRunner, ResponseWrapper, SessionManager; application boot and external provider boundary injected; actual localhost OpenAI HTTP fixture', 'observations':{}}
CONTEXT=ExecutionContext(instance_uuid='eval-instance',workspace_uuid='eval-workspace',placement_generation=1)


def make_query(mode, stream=False):
    event,_,_,_=HttpBotAdapter.model_construct(config={})._build_event({'session_id':'group-100:user-1','session_type':'group','sender':{'id':'user-1'},'message':[{'type':'Plain','text':'总结群聊'}]})
    q=Query.model_construct(query_id='eval-'+mode,launcher_type=LauncherTypes.GROUP,
        launcher_id='group-100:user-1',sender_id='user-1',message_chain=event.message_chain,message_event=event,
        adapter=NS(is_stream_output_supported=AsyncMock(return_value=stream)),pipeline_uuid='eval-pipeline',bot_uuid='eval-bot',
        pipeline_config={'ai':{'runner':{'runner':'local-agent'},'local-agent':{'model':{'primary':'primary','fallbacks':['fallback']}}},
                         'output':{'misc':{'remove-think':True,'track-function-calls':False}}},
        prompt=NS(messages=[]),messages=[],user_message=Message(role='user',content='总结群聊：甲说周五发布，乙说周四测试'),
        use_funcs=[NS(name='summarize_chat'),NS(name='create_reminder')],use_llm_model_uuid='primary',
        variables={'_fallback_model_uuids':['fallback']},resp_messages=[],resp_message_chain=[])
    object.__setattr__(q,'_execution_context',CONTEXT)
    return q


class HTTPFixtureProvider:
    def __init__(self,client,mode):self.client=client;self.mode=mode
    async def invoke_llm(self,query,model,messages,funcs,extra_args=None,remove_think=None):
        serialized=[]
        for m in messages:
            item={'role':m.role,'content':m.content}
            if m.tool_calls:item['tool_calls']=[tc.model_dump(exclude_none=True) for tc in m.tool_calls]
            if m.tool_call_id:item['tool_call_id']=m.tool_call_id
            serialized.append(item)
        res=await self.client.post('/v1/chat/completions',json={'model':model.model_entity.name,'messages':serialized,'scenario':self.mode})
        res.raise_for_status()
        return Message.model_validate(res.json()['choices'][0]['message'])


async def main():
    counts={};requests=[]
    async def completion(req):
        body=await req.json();mode=body['scenario'];model=body['model'];key=(mode,model)
        counts[key]=counts.get(key,0)+1;n=counts[key]
        requests.append({'scenario':mode,'model':model,'roles':[m['role'] for m in body['messages']]})
        if (mode=='first_fallback' and model=='primary') or (mode=='after_tool_failure' and model=='primary' and n==2):
            return aiohttp.web.json_response({'error':{'message':'secret-provider-body','type':'server_error'}},status=503)
        msg={'role':'assistant','content':'讨论结论：周五发布，周四测试。'}
        if mode in ('summary','after_tool_failure','duplicate_tool') and model=='primary' and n==1:
            name='summarize_chat' if mode=='summary' else 'create_reminder'
            calls=[{'id':'call-1','type':'function','function':{'name':name,'arguments':'{}'}}]
            if mode=='duplicate_tool': calls.append({'id':'call-2','type':'function','function':{'name':name,'arguments':'{}'}})
            msg={'role':'assistant','content':None,'tool_calls':calls}
        if mode=='protocol_text':msg={'role':'assistant','content':'<tool_call>summarize_chat({})</tool_call>'}
        return aiohttp.web.json_response({'id':'chatcmpl-fixture','object':'chat.completion','created':0,'model':model,
            'choices':[{'index':0,'message':msg,'finish_reason':'tool_calls' if msg.get('tool_calls') else 'stop'}]})
    server=aiohttp.web.Application();server.router.add_post('/v1/chat/completions',completion)
    runner=aiohttp.web.AppRunner(server);await runner.setup();site=aiohttp.web.TCPSite(runner,'127.0.0.1',0);await site.start()
    port=site._server.sockets[0].getsockname()[1]
    client=httpx.AsyncClient(base_url=f'http://127.0.0.1:{port}',timeout=2,trust_env=False)
    try:
        for mode in ('summary','first_fallback','after_tool_failure','duplicate_tool','protocol_text'):
            effects=[];raw=[];visible=[];error=None;provider=HTTPFixtureProvider(client,mode)
            models={name:NS(provider=provider,model_entity=NS(uuid=name,name=name,abilities=['func_call'],extra_args={})) for name in ('primary','fallback')}
            async def get_model(ctx,uuid):return models[uuid]
            async def execute(name,params,query):
                effects.append(name)
                return {'records':[{'name':'甲','text':'周五发布'},{'name':'乙','text':'周四测试'}]} if name=='summarize_chat' else {'receipt':'fixture-1'}
            async def emit(event,bound):return NS(event=NS(reply_message_chain=None),is_prevented_default=lambda:False)
            app=NS(logger=logging.getLogger('evaluation'),model_mgr=NS(get_model_by_uuid=get_model),
                tool_mgr=NS(execute_func_call=execute),box_service=NS(available=False),
                instance_config=NS(data={'concurrency':{'session':1}}),plugin_connector=NS(emit_event=emit))
            app.sess_mgr=SessionManager(app)
            agent=LocalAgentRunner(app,{});wrapper=ResponseWrapper(app);q=make_query(mode)
            try:
                async for msg in agent.run(q):
                    raw.append({'role':msg.role,'content':msg.content,'tool_calls':len(msg.tool_calls or [])})
                    q.resp_messages.append(msg)
                    async for result in wrapper.process(q,'eval-wrapper'):
                        if result.result_type.name=='CONTINUE' and q.resp_message_chain:
                            visible.append(str(q.resp_message_chain[-1]))
            except Exception as exc:error={'type':type(exc).__name__,'message':str(exc),'contains_fixture_secret':'secret-provider-body' in str(exc)}
            report['observations'][mode]={'model_calls':{name:counts.get((mode,name),0) for name in ('primary','fallback')},
                'tool_executions':effects,'side_effect_count':effects.count('create_reminder'),'runner_events':raw,
                'wrapper_yielded_text':visible,'error':error}
        assert report['observations']['summary']['tool_executions']==['summarize_chat']
        assert report['observations']['summary']['wrapper_yielded_text']==['讨论结论：周五发布，周四测试。']
        assert report['observations']['first_fallback']['model_calls']=={'primary':1,'fallback':1}
        assert report['observations']['after_tool_failure']['side_effect_count']==1
        assert report['observations']['after_tool_failure']['model_calls']=={'primary':2,'fallback':0}
        assert report['observations']['duplicate_tool']['side_effect_count']==2
        assert report['observations']['protocol_text']['wrapper_yielded_text']==['<tool_call>summarize_chat({})</tool_call>']

        app=NS(instance_config=NS(data={'concurrency':{'session':1}}));mgr=SessionManager(app)
        q1=make_query('session-a');q1.launcher_id='group-100';q1.sender_id='user-1'
        q2=make_query('session-b');q2.launcher_id='group-100';q2.sender_id='user-2'
        q3=make_query('session-c');q3.launcher_id='group-200';q3.sender_id='user-1'
        q4=make_query('session-d');q4.launcher_id='group-100:user-1';q4.sender_id='user-1'
        q5=make_query('session-e');q5.launcher_id='group-100:user-2';q5.sender_id='user-2'
        a,b,c,d,e=[await mgr.get_session(q) for q in (q1,q2,q3,q4,q5)]
        report['observations']['session_manager']={'same_group_different_user_shared':a is b,
            'different_group_isolated':a is not c,'composite_group_user_isolated':d is not e}
        assert a is b and a is not c and d is not e
        report['completed']=True;report['http_requests']=requests
    finally:
        await client.aclose();await runner.cleanup()
        (OUT/'runner-results.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
    print(json.dumps(report,ensure_ascii=False,indent=2))

asyncio.run(main())
