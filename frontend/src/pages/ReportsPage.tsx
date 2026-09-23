import { useEffect, useState } from 'react';
import { Button, Card, Col, Empty, Row, Segmented, Select, Space, Statistic, Typography, message } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined } from '@ant-design/icons';
import type { EnvironmentReport, Greenhouse } from '../types/domain';
import { listGreenhouses } from '../api/greenhouse';
import { getReport } from '../api/monitoring';
import ReportChart from '../components/charts/ReportChart';
import { exportReportPdf } from '../utils/export';

export default function ReportsPage() {
  const [greenhouses, setGreenhouses] = useState<Greenhouse[]>([]);
  const [selected, setSelected] = useState<number>();
  const [range, setRange] = useState('day');
  const [report, setReport] = useState<EnvironmentReport>();

  useEffect(() => {
    void listGreenhouses()
      .then((rows) => {
        setGreenhouses(rows);
        setSelected(rows[0]?.id);
      })
      .catch(() => message.error('无法获取温室列表'));
  }, []);

  useEffect(() => {
    if (selected) void getReport(selected, range).then(setReport).catch(() => message.error('报告生成失败'));
  }, [selected, range]);

  if (!greenhouses.length) return <Empty />;

  return (
    <Space direction="vertical" size={20} style={{ width: '100%' }}>
      <Typography.Title level={2}>环境分析报告</Typography.Title>
      <Card>
        <Space wrap>
          <Select
            value={selected}
            style={{ width: 220 }}
            options={greenhouses.map((item) => ({ value: item.id, label: item.name }))}
            onChange={setSelected}
          />
          <Segmented
            value={range}
            options={[
              { label: '日报', value: 'day' },
              { label: '周报', value: 'week' },
              { label: '月报', value: 'month' },
            ]}
            onChange={setRange}
          />
          <Button onClick={() => report && exportReportPdf(report)}>导出 PDF</Button>
        </Space>
      </Card>
      {report && (
        <>
          <Row gutter={[16, 16]}>
            <Col xs={24} md={8}>
              <Card>
                <Statistic title="报告周期内报警总数" value={report.alerts} />
              </Card>
            </Col>
            <Col xs={12} md={8}>
              <Card>
                <Statistic
                  title="待处理报警"
                  value={report.pendingAlerts}
                  prefix={<ClockCircleOutlined style={{ color: '#faad14' }} />}
                  valueStyle={{ color: '#d48806' }}
                />
              </Card>
            </Col>
            <Col xs={12} md={8}>
              <Card>
                <Statistic
                  title="已处理报警"
                  value={report.handledAlerts}
                  prefix={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
                  valueStyle={{ color: '#389e0d' }}
                />
              </Card>
            </Col>
            <Col xs={24} md={24}>
              <Card title="环境指标雷达图">
                <ReportChart report={report} />
              </Card>
            </Col>
          </Row>
          <Card title="指标摘要">
            <Row gutter={[16, 16]}>
              {Object.entries(report.metrics).map(([key, value]) => (
                <Col key={key} xs={12} md={6}>
                  <Statistic title={key} value={value.average} precision={1} suffix={value.unit} />
                  <small>
                    最低 {value.min.toFixed(1)} · 最高 {value.max.toFixed(1)}
                  </small>
                </Col>
              ))}
            </Row>
          </Card>
        </>
      )}
    </Space>
  );
}
