"""Run the unmodified LangBot HTTP Bot adapter, with synthetic listeners.
The listener is a test seam, not the real pipeline. Callback HTTP is real localhost.
"""
import asyncio
import json
import os
import time
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import AsyncMock

import aiohttp.web
from quart import Quart, request
from langbot.pkg.platform.sources.http_bot import HttpBotAdapter
from langbot.pkg.platform.sources import http_bot_signing as signing
from langbot.pkg.utils import httpclient
from langbot_plugin.api.entities.builtin.platform import events, message

OUT = Path(os.environ.get('AI_EVAL_OUTPUT', str(Path(__file__).parent)))
OUT.mkdir(parents=True, exist_ok=True)
SECRET = 'synthetic-langbot-evaluation-secret'
report = {'scope': 'real HttpBotAdapter; Quart in-process HTTP ingress; synthetic pipeline listener; real localhost callback HTTP', 'observations': {}}


def make_adapter(callback_url, timeout=1):
    return HttpBotAdapter.model_construct(
        config={'inbound_secret': SECRET, 'signature_required': True,
                'callback_url': callback_url, 'callback_timeout': timeout, 'callback_max_retries': 1},
        logger=SimpleNamespace(warning=AsyncMock(), error=AsyncMock()),
        bot_uuid='evaluation-bot', listeners={}, outbound_states={},
        idempotency_cache={}, sync_waiters={}, inbound_tasks=set())


def chain(text):
    return message.MessageChain.model_validate([{'type':'Plain', 'text':text}])


def bind_app(adapter):
    app = Quart(__name__)
    @app.post('/bots/evaluation-bot')
    @app.post('/bots/evaluation-bot/<path:subpath>')
    async def webhook(subpath=''):
        return await adapter.handle_unified_webhook('evaluation-bot', subpath, request)
    return app.test_client()


async def post(client, *, sid='group-100:user-1', text='总结群聊', key=None, path='', raw=None, timestamp=None, bad=False, missing=False):
    body = raw if raw is not None else json.dumps({'session_id':sid, 'session_type':'group',
        'sender':{'id':'user-1'}, 'message':[{'type':'Plain','text':text}]},ensure_ascii=False).encode()
    ts,sig = signing.sign(SECRET,body,timestamp)
    headers={'Content-Type':'application/json'}
    if not missing: headers.update({signing.HEADER_TIMESTAMP:ts,signing.HEADER_SIGNATURE:'sha256=bad' if bad else sig})
    if key: headers[signing.HEADER_IDEMPOTENCY]=key
    return await client.post('/bots/evaluation-bot'+('/'+path if path else ''),data=body,headers=headers)


async def main():
    callbacks=[]
    callback_statuses=[]
    async def receiver(req):
        body=await req.read()
        valid,reason=signing.verify(SECRET,body,req.headers.get(signing.HEADER_TIMESTAMP),req.headers.get(signing.HEADER_SIGNATURE))
        callbacks.append({'payload':json.loads(body),'signature_valid':valid,'reason':reason})
        return aiohttp.web.Response(status=callback_statuses.pop(0) if callback_statuses else 200)
    app=aiohttp.web.Application();app.router.add_post('/callback',receiver)
    web_runner=aiohttp.web.AppRunner(app);await web_runner.setup()
    site=aiohttp.web.TCPSite(web_runner,'127.0.0.1',0);await site.start()
    port=site._server.sockets[0].getsockname()[1]
    url=f'http://127.0.0.1:{port}/callback'
    adapters=[]
    try:
        adapter=make_adapter(url);adapters.append(adapter);client=bind_app(adapter)
        received=[]
        async def listener(event, adapter): received.append(event)
        adapter.register_listener(events.GroupMessage,listener)
        statuses={}
        for name, kwargs in [('missing',{'missing':True}),('invalid',{'bad':True}),('expired',{'timestamp':int(time.time())-301}),('valid',{})]:
            res=await post(client,**kwargs);statuses[name]=res.status_code
        assert statuses=={'missing':401,'invalid':401,'expired':401,'valid':202}
        r1=await post(client,key='same-event');r2=await post(client,key='same-event')
        n1=await post(client);n2=await post(client)
        await asyncio.sleep(0)
        assert (r1.status_code,r2.status_code,n1.status_code,n2.status_code)==(202,409,202,202)
        malformed=json.dumps({'session_id':'bad','session_type':'group','message':42}).encode()
        m1=await post(client,key='invalid-reserve',raw=malformed);m2=await post(client,key='invalid-reserve')
        report['observations']['inbound']={'signature_statuses':statuses,'duplicate_with_key':[r1.status_code,r2.status_code],
            'duplicate_without_key':[n1.status_code,n2.status_code], 'malformed_then_corrected_same_key':[m1.status_code,m2.status_code],
            'received_events':len(received)}
        assert (m1.status_code,m2.status_code)==(400,409)

        event=received[0]
        callback_statuses[:]=[503,200]
        await adapter.reply_message(event,chain('周五发布，周四测试'))
        state=adapter.outbound_states[adapter.get_launcher_id(event)]
        await asyncio.wait_for(state.queue.join(),4)
        assert len(callbacks)==2 and callbacks[0]['payload']==callbacks[1]['payload']
        retry_payload=callbacks[0]['payload']
        before=len(callbacks);callback_statuses[:]=[429]
        await adapter.reply_message(event,chain('限流测试'))
        await asyncio.wait_for(state.queue.join(),4)
        assert len(callbacks)-before==1
        report['observations']['callback']={'http_503_attempts':2,'retry_payload_identical':True,
            'signature_valid_on_all':all(c['signature_valid'] for c in callbacks),
            'dedup_fields':['session_id','reply_to','sequence'], 'payload_fields':sorted(retry_payload),
            'http_429_attempts':len(callbacks)-before,
            'sequence_restart_for_new_final_reply':callbacks[-1]['payload']['sequence']}

        slow=make_adapter(url);adapters.append(slow);slow_client=bind_app(slow)
        first_release=asyncio.Event();second_release=asyncio.Event();first_started=asyncio.Event();second_started=asyncio.Event()
        active={'count':0,'completed':0}
        async def slow_listener(event, adapter):
            active['count']+=1
            number=active['count']
            if number==1:
                first_started.set();await first_release.wait()
                await adapter.reply_message(event,chain('第一轮迟到答案：周五发布'))
            else:
                second_started.set();await second_release.wait()
                await adapter.reply_message(event,chain('第二轮答案：周四测试'))
            active['completed']+=1
        slow.register_listener(events.GroupMessage,slow_listener)
        start=time.monotonic();r=await post(slow_client,path='sync',key='slow-1')
        payload=await r.get_json();duration=time.monotonic()-start
        running=len(slow.inbound_tasks)
        assert r.status_code==200 and payload['data']['message']==[] and running==1
        second=asyncio.create_task(post(slow_client,path='sync',key='slow-2',text='周几测试'))
        await asyncio.wait_for(second_started.wait(),1)
        first_release.set()
        rsecond=await asyncio.wait_for(second,1);psecond=await rsecond.get_json()
        assert psecond['data']['message'][0]['text']=='第一轮迟到答案：周五发布'
        second_release.set();await asyncio.sleep(0.1)
        second_state=slow.outbound_states['group-100:user-1'];await asyncio.wait_for(second_state.queue.join(),2)
        report['observations']['sync_timeout']={'elapsed_seconds':round(duration,3),'http_status':r.status_code,'code':payload['code'],
            'returned_parts':payload['data']['message'],'inbound_tasks_still_running':running,
            'later_sync_received_old_turn_reply':psecond['data']['message'][0]['text'],
            'second_reply_relocated_to_callback':callbacks[-1]['payload']['message'][0]['text'],
            'completed_listener_count':active['completed']}

        cancel=make_adapter(url);adapters.append(cancel);cancel_client=bind_app(cancel)
        started=asyncio.Event();release=asyncio.Event();completed=asyncio.Event()
        async def cancel_listener(event, adapter):
            started.set();await release.wait();completed.set()
            await adapter.reply_message(event,chain('取消后仍完成'))
        cancel.register_listener(events.GroupMessage,cancel_listener)
        task=asyncio.create_task(post(cancel_client,path='sync',key='cancel'))
        await asyncio.wait_for(started.wait(),1);task.cancel()
        try: await task
        except asyncio.CancelledError: pass
        running=len(cancel.inbound_tasks);release.set();await asyncio.wait_for(completed.wait(),1)
        state=cancel.outbound_states['group-100:user-1'];await asyncio.wait_for(state.queue.join(),2)
        report['observations']['request_cancel']={'listener_tasks_surviving_cancel':running,'completed_after_cancel':completed.is_set()}

        same_group_events=[]
        for user in ('a','b'):
            e,sid,stype,mid=adapter._build_event({'session_id':'group-100','session_type':'group','sender':{'id':user},'message':[{'type':'Plain','text':'hi'}]})
            same_group_events.append({'sender':str(e.sender.id),'launcher':adapter.get_launcher_id(e)})
        report['observations']['adapter_identity']={'different_users_same_group':same_group_events,
            'per_user_isolation_requires_composite_session_id':True}
        report['completed']=True
    finally:
        for a in adapters: await a.kill()
        await httpclient.close_all();await web_runner.cleanup()
        (OUT/'http-results.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
    print(json.dumps(report,ensure_ascii=False,indent=2))

asyncio.run(main())
