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

import React, { useContext } from 'react';
import {
  Space,
  Button,
  Select,
  Switch,
  InputNumber,
  Typography,
  Spin,
} from '@douyinfe/semi-ui';
import { IconRefresh, IconHistogram } from '@douyinfe/semi-icons';
import { UserContext } from '../../context/User';
import { useCacheMetrics } from '../../hooks/cache/useCacheMetrics';
import StatsCards from './StatsCards';
import ChartsPanel from './ChartsPanel';
import ChannelTable from './ChannelTable';
import WarmupPanel from './WarmupPanel';

const { Title, Text } = Typography;

const CacheAnalytics = () => {
  // ========== Context ==========
  const [userState] = useContext(UserContext);

  // ========== 数据管理 ==========
  const cacheMetrics = useCacheMetrics(userState);

  return (
    <div className='h-full'>
      {/* Header */}
      <div className='px-6 py-4 border-b'>
        <div className='flex justify-between items-center'>
          <div className='flex items-center gap-3'>
            <IconHistogram size='extra-large' />
            <div>
              <Title heading={3} style={{ margin: 0 }}>
                {cacheMetrics.t('缓存分析')}
              </Title>
              <Text type='tertiary'>{cacheMetrics.t('Claude缓存命中率与成本节省分析')}</Text>
            </div>
          </div>

          <Space>
            {/* Period Selector */}
            <Select
              value={cacheMetrics.period}
              onChange={cacheMetrics.handlePeriodChange}
              style={{ width: 140 }}
              placeholder={cacheMetrics.t('选择时间范围')}
            >
              {cacheMetrics.periodOptions.map((option) => (
                <Select.Option key={option.value} value={option.value}>
                  {option.label}
                </Select.Option>
              ))}
            </Select>

            {/* 🔥 Cost Unit Selector */}
            <Select
              value={cacheMetrics.costUnit}
              onChange={cacheMetrics.handleCostUnitChange}
              style={{ width: 120 }}
              placeholder={cacheMetrics.t('成本单位')}
            >
              <Select.Option value='quota'>额度</Select.Option>
              <Select.Option value='tokens'>Tokens</Select.Option>
              <Select.Option value='usd'>USD ($)</Select.Option>
              <Select.Option value='cny'>CNY (¥)</Select.Option>
            </Select>

            {/* Auto Refresh Toggle */}
            <Space>
              <Text>{cacheMetrics.t('自动刷新')}</Text>
              <Switch
                checked={cacheMetrics.autoRefresh}
                onChange={cacheMetrics.handleAutoRefreshToggle}
              />
            </Space>

            {/* Refresh Interval */}
            {cacheMetrics.autoRefresh && (
              <Space>
                <InputNumber
                  value={cacheMetrics.refreshInterval / 1000}
                  onChange={cacheMetrics.handleRefreshIntervalChange}
                  min={10}
                  max={300}
                  step={10}
                  suffix='s'
                  style={{ width: 100 }}
                />
              </Space>
            )}

            {/* Refresh Button */}
            <Button
              icon={<IconRefresh />}
              onClick={cacheMetrics.refresh}
              loading={cacheMetrics.loading}
              theme='solid'
              type='primary'
            >
              {cacheMetrics.t('刷新')}
            </Button>
          </Space>
        </div>
      </div>

      {/* Content */}
      <Spin spinning={cacheMetrics.loading}>
        {/* Stats Cards */}
        <div className='mb-4'>
          <StatsCards statsCards={cacheMetrics.statsCards} />
        </div>

        {/* Charts Panel */}
        <div className='mb-4'>
          <ChartsPanel
            chartData={cacheMetrics.chartData}
            activeChartTab={cacheMetrics.activeChartTab}
            setActiveChartTab={cacheMetrics.setActiveChartTab}
            period={cacheMetrics.period}
            interval={cacheMetrics.interval}
            costUnit={cacheMetrics.costUnit}
            unitLabel={cacheMetrics.unitLabel}
            t={cacheMetrics.t}
          />
        </div>

        {/* Admin-Only Sections */}
        {cacheMetrics.isAdminUser && (
          <>
            {/* Channel Metrics Table */}
            <div className='mb-4'>
              <ChannelTable
                channelMetrics={cacheMetrics.channelMetrics}
                loading={cacheMetrics.loading}
                costUnit={cacheMetrics.costUnit}
                t={cacheMetrics.t}
              />
            </div>

            {/* Warmup Status Panel */}
            <div className='mb-4'>
              <WarmupPanel
                warmupStatus={cacheMetrics.warmupStatus}
                loading={cacheMetrics.loading}
                t={cacheMetrics.t}
              />
            </div>
          </>
        )}
      </Spin>
    </div>
  );
};

export default CacheAnalytics;