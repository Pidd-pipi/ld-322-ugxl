import { useState } from 'react';
import { Button, Card, Input, List, Modal, Space, Tag, Typography, message } from 'antd';
import { ExclamationCircleFilled } from '@ant-design/icons';
import type { Alert } from '../../types/domain';
import StatusTag from '../common/StatusTag';
import { handleAlert } from '../../api/monitoring';

const { Text, Paragraph } = Typography;

export default function AlertList({ alerts, onUpdated }: { alerts: Alert[]; onUpdated: () => void }) {
  const [targetId, setTargetId] = useState<number>();
  const [note, setNote] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const close = () => {
    setTargetId(undefined);
    setNote('');
  };

  const submit = async () => {
    if (targetId === undefined) return;
    if (!note.trim()) {
      message.warning('请填写处理说明，说明为空时不能处理报警');
      return;
    }
    setSubmitting(true);
    try {
      await handleAlert(targetId, note.trim());
      message.success('报警已处理并记录');
      close();
      onUpdated();
    } catch (err: any) {
      message.error(err?.response?.data?.message || '处理失败，请先登录演示账号');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card
      title="实时报警"
      extra={<Button size="small" onClick={onUpdated}>刷新</Button>}
    >
      <List
        size="small"
        locale={{ emptyText: '当前没有报警' }}
        dataSource={alerts.slice(0, 6)}
        renderItem={(item) => (
          <List.Item
            actions={
              item.status === 'pending'
                ? [
                    <Button
                      type="link"
                      key="handle"
                      onClick={() => setTargetId(item.id)}
                    >
                      标记处理
                    </Button>,
                  ]
                : []
            }
          >
            <List.Item.Meta
              avatar={item.status === 'pending' ? <ExclamationCircleFilled style={{ color: '#faad14', fontSize: 18, marginTop: 4 }} /> : undefined}
              title={
                <Space wrap size={6}>
                  <StatusTag status={item.level} />
                  <Text>{item.message}</Text>
                  {item.status === 'handled' && <Tag color="green">已处理</Tag>}
                </Space>
              }
              description={
                <Space direction="vertical" size={2}>
                  <Text type="secondary">报警时间：{new Date(item.createdAt).toLocaleString('zh-CN')}</Text>
                  {item.status === 'handled' && (
                    <>
                      <Text>
                        处理人：{item.handledBy || '未知'} · 处理时间：{item.handledAt ? new Date(item.handledAt).toLocaleString('zh-CN') : '-'}
                      </Text>
                      <Paragraph style={{ margin: 0 }}>处理说明：{item.handleNote}</Paragraph>
                    </>
                  )}
                </Space>
              }
            />
          </List.Item>
        )}
      />
      <Modal
        title="处理报警"
        open={targetId !== undefined}
        onOk={submit}
        okText="确认处理"
        cancelText="取消"
        confirmLoading={submitting}
        onCancel={close}
        okButtonProps={{ disabled: !note.trim() }}
        destroyOnClose
      >
        <Paragraph type="secondary" style={{ marginBottom: 8 }}>
          处理人将按当前登录账号记录，处理说明不能为空，提交后不可修改。
        </Paragraph>
        <Input.TextArea
          rows={4}
          maxLength={500}
          showCount
          placeholder="请填写采取的措施，例如：已开启循环风机降温，30 分钟后复测温度恢复正常。"
          value={note}
          onChange={(e) => setNote(e.target.value)}
          onPressEnter={(e) => {
            if (e.ctrlKey || e.metaKey) void submit();
          }}
        />
      </Modal>
    </Card>
  );
}
