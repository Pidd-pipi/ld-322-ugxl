import { useState } from 'react'; import { Button, Card, Input, List, Modal, Typography, message } from 'antd'; import type { Alert } from '../../types/domain'; import StatusTag from '../common/StatusTag'; import { handleAlert } from '../../api/monitoring';
export default function AlertList({alerts,onUpdated}:{alerts:Alert[];onUpdated:()=>void}){
  const [target,setTarget]=useState<Alert>();const [note,setNote]=useState('');const [submitting,setSubmitting]=useState(false);
  const open=(item:Alert)=>{setTarget(item);setNote('')};
  const resolve=async()=>{if(!target)return;const trimmed=note.trim();if(!trimmed){message.warning('请填写处理说明后再提交');return}setSubmitting(true);try{await handleAlert(target.id,trimmed);message.success('报警已标记为已处理');setTarget(undefined);onUpdated()}catch{message.error('处理失败，请先登录演示账号')}finally{setSubmitting(false)}};
  const describe=(item:Alert)=>{if(item.status!=='handled')return new Date(item.createdAt).toLocaleString('zh-CN');return <span>{new Date(item.createdAt).toLocaleString('zh-CN')}<br/><Typography.Text type="secondary">处理说明：{item.handleNote||'—'} · 处理人：{item.handledBy||'—'} · 处理时间：{item.handledAt?new Date(item.handledAt).toLocaleString('zh-CN'):'—'}</Typography.Text></span>};
  return <Card title="实时报警" extra={<Button size="small" onClick={onUpdated}>刷新</Button>}>
    <List size="small" locale={{emptyText:'当前没有报警'}} dataSource={alerts.slice(0,6)} renderItem={item=><List.Item actions={item.status==='pending'?[<Button type="link" key="handle" onClick={()=>open(item)}>标记处理</Button>]:[]}><List.Item.Meta title={<><StatusTag status={item.level}/>{item.message}</>} description={describe(item)}/></List.Item>}/>
    <Modal title="处理报警" open={!!target} onOk={resolve} onCancel={()=>setTarget(undefined)} confirmLoading={submitting} okText="确认处理" cancelText="取消" destroyOnClose>
      <Typography.Paragraph>{target?.message}</Typography.Paragraph>
      <Input.TextArea rows={3} maxLength={500} showCount placeholder="请填写处理说明（必填），例如：已开启循环风机降温" value={note} onChange={event=>setNote(event.target.value)}/>
    </Modal>
  </Card>
}
