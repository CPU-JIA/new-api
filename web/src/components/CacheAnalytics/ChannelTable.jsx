/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Card, Table, Tag, Empty, Typography } from '@douyinfe/semi-ui';
import { IconServerStroked } from '@douyinfe/semi-icons';

const { Text } = Typography;

const ChannelTable = ({ channelMetrics, loading, costUnit, t }) => {
  // 🔥 根据costUnit动态获取显示值和单位标签
  const getCostDisplay = (record) => {
    const quotaValue = record.cost_saved_quota || record.total_cost_saved || 0;
    const usdValue = record.cost_saved_usd || 0;
    const cnyValue = record.cost_saved_cny || 0;
    const tokensValue = record.cost_saved_tokens || 0;

    switch (costUnit) {
      case 'usd':
        return {
          value: usdValue.toFixed(6),
          unit: 'USD',
          prefix: '$',
        };
      case 'cny':
        return {
          value: cnyValue.toFixed(4),
          unit: 'CNY',
          prefix: '¥',
        };
      case 'tokens':
        return {
          value: Math.round(tokensValue).toLocaleString(),
          unit: 'Tokens',
          prefix: '',
        };
      case 'quota':
      default:
        return {
          value: quotaValue.toFixed(2),
          unit: '额度',
          prefix: '',
        };
    }
  };

  const columns = [
    {
      title: t('渠道ID'),
      dataIndex: 'channel_id',
      key: 'channel_id',
      width: 100,
      sorter: (a, b) => a.channel_id - b.channel_id,
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: t('渠道名称'),
      dataIndex: 'channel_name',
      key: 'channel_name',
      ellipsis: true,
      render: (text) => text || t('未命名'),
    },
    {
      title: t('请求数'),
      dataIndex: 'total_requests',
      key: 'total_requests',
      width: 120,
      sorter: (a, b) => a.total_requests - b.total_requests,
      render: (text) => (text || 0).toLocaleString(),
    },
    {
      title: t('缓存命中率'),
      dataIndex: 'avg_cache_hit_rate',
      key: 'avg_cache_hit_rate',
      width: 150,
      sorter: (a, b) => a.avg_cache_hit_rate - b.avg_cache_hit_rate,
      render: (rate) => {
        const percentage = ((rate || 0) * 100).toFixed(2);
        const color =
          rate >= 0.8 ? 'green' : rate >= 0.5 ? 'amber' : 'red';
        return (
          <Tag color={color} size='large'>
            {percentage}%
          </Tag>
        );
      },
    },
    {
      title: t('节省成本'),
      dataIndex: 'total_cost_saved',
      key: 'total_cost_saved',
      width: 150,
      sorter: (a, b) => {
        const aCost = a.cost_saved_quota || a.total_cost_saved || 0;
        const bCost = b.cost_saved_quota || b.total_cost_saved || 0;
        return aCost - bCost;
      },
      render: (_, record) => {
        const display = getCostDisplay(record);
        const rawValue = record.cost_saved_quota || record.total_cost_saved || 0;
        return (
          <Text type={rawValue > 0 ? 'success' : 'tertiary'}>
            {display.prefix}{display.value} {display.unit}
          </Text>
        );
      },
    },
    {
      title: t('Warmup状态'),
      dataIndex: 'warmup_enabled',
      key: 'warmup_enabled',
      width: 130,
      filters: [
        { text: t('已启用'), value: true },
        { text: t('未启用'), value: false },
      ],
      onFilter: (value, record) => record.warmup_enabled === value,
      render: (enabled) => {
        return enabled ? (
          <Tag color='green'>{t('已启用')}</Tag>
        ) : (
          <Tag color='grey'>{t('未启用')}</Tag>
        );
      },
    },
    {
      title: t('5分钟请求数'),
      dataIndex: 'request_count_5min',
      key: 'request_count_5min',
      width: 140,
      sorter: (a, b) => a.request_count_5min - b.request_count_5min,
      render: (count) => {
        const color = count >= 10 ? 'green' : count > 0 ? 'amber' : 'grey';
        return <Tag color={color}>{count || 0}</Tag>;
      },
    },
    {
      title: t('最后Warmup'),
      dataIndex: 'last_warmup',
      key: 'last_warmup',
      width: 160,
      render: (timestamp) => {
        if (!timestamp || timestamp === 0) {
          return <Text type='tertiary'>{t('从未')}</Text>;
        }
        const date = new Date(timestamp * 1000);
        const now = Date.now();
        const diff = now - date.getTime();
        const minutes = Math.floor(diff / 60000);

        if (minutes < 1) {
          return <Text type='success'>{t('刚刚')}</Text>;
        } else if (minutes < 60) {
          return <Text type='success'>{minutes} {t('分钟前')}</Text>;
        } else {
          return (
            <Text type='tertiary'>
              {date.toLocaleString('zh-CN', {
                month: 'numeric',
                day: 'numeric',
                hour: 'numeric',
                minute: 'numeric',
              })}
            </Text>
          );
        }
      },
    },
  ];

  return (
    <Card
      className='!rounded-2xl shadow-sm'
      title={
        <div className='flex items-center gap-2'>
          <IconServerStroked />
          {t('渠道缓存详情')}
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      {channelMetrics && channelMetrics.length > 0 ? (
        <Table
          columns={columns}
          dataSource={channelMetrics}
          rowKey='channel_id'
          loading={loading}
          pagination={{
            pageSize: 10,
            showSizeChanger: true,
            pageSizeOpts: [10, 20, 50],
          }}
          bordered={false}
        />
      ) : (
        <div className='p-8'>
          <Empty
            title={t('暂无数据')}
            description={t('当前时间范围内没有渠道缓存数据')}
          />
        </div>
      )}
    </Card>
  );
};

export default ChannelTable;