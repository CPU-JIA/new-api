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

import React, { useMemo, useState } from 'react';
import { Card, Table, Tag, Empty, Badge, Typography, Space, Switch } from '@douyinfe/semi-ui';
import { IconPulse, IconBolt } from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const WarmupPanel = ({ warmupStatus, loading, t }) => {
  // 🔥 状态：显示全部渠道还是仅活跃渠道
  const [showAll, setShowAll] = useState(false);

  const channels = warmupStatus?.channels || [];
  const totalChannels = warmupStatus?.total_channels || 0;

  // Calculate active warmup count
  const activeWarmupCount = channels.filter((ch) => ch.warmup_enabled).length;

  // 🔥 根据showAll状态过滤显示的渠道
  const displayedChannels = useMemo(() => {
    if (showAll) {
      return channels; // 显示全部
    }
    return channels.filter((ch) => ch.warmup_enabled); // 仅显示活跃
  }, [channels, showAll]);

  const columns = [
    {
      title: t('渠道ID'),
      dataIndex: 'channel_id',
      key: 'channel_id',
      width: 100,
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
      title: t('Warmup状态'),
      dataIndex: 'warmup_enabled',
      key: 'warmup_enabled',
      width: 130,
      filters: [
        { text: t('活跃'), value: true },
        { text: t('非活跃'), value: false },
      ],
      onFilter: (value, record) => record.warmup_enabled === value,
      render: (enabled, record) => {
        if (!enabled) {
          return <Tag color='grey'>{t('非活跃')}</Tag>;
        }

        // Check if warmup is stale (last warmup > 5 minutes ago)
        const now = Date.now() / 1000;
        const lastWarmup = record.last_warmup;
        const isStale = lastWarmup > 0 && (now - lastWarmup) > 300;

        return (
          <Badge dot={!isStale} count={isStale ? '!' : undefined}>
            <Tag color={isStale ? 'amber' : 'green'}>
              <IconBolt /> {t('活跃')}
            </Tag>
          </Badge>
        );
      },
    },
    {
      title: t('缓存TTL'),
      dataIndex: 'ttl',
      key: 'ttl',
      width: 100,
      filters: [
        { text: '5分钟', value: '5m' },
        { text: '1小时', value: '1h' },
      ],
      onFilter: (value, record) => record.ttl === value,
      render: (ttl) => {
        if (!ttl) {
          return <Tag color='grey'>未配置</Tag>;
        }
        const color = ttl === '1h' ? 'purple' : 'blue';
        const label = ttl === '1h' ? '1小时' : '5分钟';
        return <Tag color={color}>{label}</Tag>;
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
        return (
          <Tag color={color} size='large'>
            {count || 0}
          </Tag>
        );
      },
    },
    {
      title: t('预热次数'),
      dataIndex: 'warmup_count',
      key: 'warmup_count',
      width: 110,
      sorter: (a, b) => (a.warmup_count || 0) - (b.warmup_count || 0),
      render: (count) => {
        const numCount = count || 0;
        let color = 'grey';
        if (numCount > 10) {
          color = 'green';
        } else if (numCount > 0) {
          color = 'amber';
        }
        return (
          <Tag color={color} size='large'>
            {numCount}
          </Tag>
        );
      },
    },
    {
      title: t('最后请求'),
      dataIndex: 'last_request',
      key: 'last_request',
      width: 160,
      render: (timestamp) => {
        if (!timestamp || timestamp === 0) {
          return <Text type='tertiary'>{t('从未')}</Text>;
        }

        const date = new Date(timestamp * 1000);
        const now = Date.now();
        const diff = now - date.getTime();
        const seconds = Math.floor(diff / 1000);
        const minutes = Math.floor(seconds / 60);

        if (seconds < 60) {
          return <Text type='success'>{seconds} {t('秒前')}</Text>;
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
    {
      title: t('最后Warmup'),
      dataIndex: 'last_warmup',
      key: 'last_warmup',
      width: 160,
      render: (timestamp) => {
        if (!timestamp || timestamp === 0) {
          return <Text type='tertiary'>{t('等待中')}</Text>;
        }

        const date = new Date(timestamp * 1000);
        const now = Date.now();
        const diff = now - date.getTime();
        const seconds = Math.floor(diff / 1000);
        const minutes = Math.floor(seconds / 60);

        // Highlight if warmup is recent (< 1 minute)
        if (seconds < 60) {
          return <Text type='success'><IconBolt /> {seconds} {t('秒前')}</Text>;
        } else if (minutes < 60) {
          return <Text>{minutes} {t('分钟前')}</Text>;
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
    {
      title: t('ROI状态'),
      dataIndex: 'consecutive_low_roi',
      key: 'consecutive_low_roi',
      width: 120,
      filters: [
        { text: '良好', value: 0 },
        { text: '监控中', value: 1 },
        { text: '风险', value: 2 },
      ],
      onFilter: (value, record) => {
        const roi = record.consecutive_low_roi || 0;
        if (value === 0) return roi === 0;
        if (value === 1) return roi === 1 || roi === 2;
        if (value === 2) return roi >= 3;
        return false;
      },
      render: (consecutiveLowROI, record) => {
        const roi = consecutiveLowROI || 0;
        if (!record.warmup_enabled) {
          return <Tag color='grey'>未启用</Tag>;
        }
        if (roi === 0) {
          return <Tag color='green'>良好</Tag>;
        } else if (roi >= 3) {
          return (
            <Tag color='red'>
              已禁用
            </Tag>
          );
        } else {
          return (
            <Tag color='amber'>
              监控中({roi}/3)
            </Tag>
          );
        }
      },
    },
    {
      title: t('监测窗口开始'),
      dataIndex: 'window_start',
      key: 'window_start',
      width: 160,
      render: (timestamp) => {
        if (!timestamp || timestamp === 0) {
          return <Text type='tertiary'>-</Text>;
        }

        const date = new Date(timestamp * 1000);
        return (
          <Text size='small' type='tertiary'>
            {date.toLocaleString('zh-CN', {
              hour: 'numeric',
              minute: 'numeric',
              second: 'numeric',
            })}
          </Text>
        );
      },
    },
  ];

  return (
    <Card
      className='!rounded-2xl shadow-sm'
      title={
        <div className='flex items-center justify-between w-full'>
          <Space>
            <IconPulse />
            <span>{t('Cache Warmer 实时状态')}</span>
          </Space>
          <Space>
            {/* 🔥 显示模式切换开关 - 无文字版本 */}
            <Text type='tertiary' size='small'>
              {showAll ? '显示全部渠道' : '仅显示活跃'}
            </Text>
            <Switch
              checked={showAll}
              onChange={setShowAll}
              style={{ marginRight: 8 }}
            />
            <Tag color='blue'>
              {t('总渠道')}: {totalChannels}
            </Tag>
            <Tag color='green'>
              {t('活跃Warmup')}: {activeWarmupCount}
            </Tag>
          </Space>
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      {displayedChannels && displayedChannels.length > 0 ? (
        <Table
          columns={columns}
          dataSource={displayedChannels}
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
            description={
              showAll
                ? t('CacheWarmer服务未检测到任何渠道')
                : t('暂无活跃的Warmup渠道，切换到"全部渠道"查看所有渠道')
            }
          />
        </div>
      )}

      {/* Warmup Service Info */}
      <div className='px-6 py-4 bg-gray-50 border-t'>
        <Space align='center'>
          <Badge dot count={activeWarmupCount > 0 ? undefined : 0}>
            <Tag color={activeWarmupCount > 0 ? 'green' : 'grey'} size='large'>
              {t('Warmup服务')}: {activeWarmupCount > 0 ? t('运行中') : t('待机')}
            </Tag>
          </Badge>
          <Text type='tertiary' size='small'>
            {t('自动检测5分钟内QPS≥10的渠道，每4分钟执行一次Warmup')}
          </Text>
          {!showAll && activeWarmupCount === 0 && (
            <Text type='warning' size='small'>
              ({t('切换到"全部渠道"查看所有渠道状态')})
            </Text>
          )}
        </Space>
      </div>
    </Card>
  );
};

export default WarmupPanel;