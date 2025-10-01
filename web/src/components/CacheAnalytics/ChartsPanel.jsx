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

import React, { useMemo } from 'react';
import { Card, Tabs, TabPane, Empty } from '@douyinfe/semi-ui';
import { IconHistogram } from '@douyinfe/semi-icons';
import { VChart } from '@visactor/react-vchart';

const CHART_CONFIG = {
  mode: 'desktop-browser',
  larkMap: {
    enable: false,
  },
  tooltip: {
    visible: true,
  },
};

const ChartsPanel = ({
  chartData,
  activeChartTab,
  setActiveChartTab,
  period,
  interval,
  costUnit,
  unitLabel,
  t,
}) => {
  // Prepare data for charts
  const chartDataPoints = useMemo(() => {
    if (!chartData || !chartData.timestamps) {
      return [];
    }

    // 🔥 Select cost array based on costUnit
    let costArray;
    switch (costUnit) {
      case 'usd':
        costArray = chartData.cost_saved_usd || [];
        break;
      case 'cny':
        costArray = chartData.cost_saved_cny || [];
        break;
      case 'tokens':
        costArray = chartData.cost_saved_tokens || [];
        break;
      case 'quota':
      default:
        costArray = chartData.cost_saved_quota || chartData.cost_saved || [];
        break;
    }

    return chartData.timestamps.map((timestamp, idx) => ({
      time: new Date(timestamp * 1000).toLocaleString('zh-CN', {
        month: 'numeric',
        day: 'numeric',
        hour: period === '1h' ? 'numeric' : undefined,
        minute: period === '1h' ? 'numeric' : undefined,
      }),
      timestamp: timestamp,
      cacheHitRate: ((chartData.cache_hit_rates[idx] || 0) * 100).toFixed(2),
      costSaved: (costArray[idx] || 0).toFixed(costUnit === 'tokens' ? 0 : costUnit === 'usd' ? 6 : costUnit === 'cny' ? 4 : 2),
    }));
  }, [chartData, period, costUnit]);

  // Cache Hit Rate Chart Spec
  const hitRateSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'hitRate',
          values: chartDataPoints,
        },
      ],
      xField: 'time',
      yField: 'cacheHitRate',
      seriesField: 'type',
      line: {
        style: {
          lineWidth: 3,
          lineCap: 'round',
        },
      },
      point: {
        visible: true,
        style: {
          size: 5,
          fill: 'white',
          stroke: '#4096ff',
          lineWidth: 2,
        },
      },
      axes: [
        {
          orient: 'left',
          title: {
            visible: true,
            text: t('缓存命中率 (%)'),
          },
          min: 0,
          max: 100,
        },
        {
          orient: 'bottom',
          title: {
            visible: true,
            text: t('时间'),
          },
          label: {
            autoRotate: true,
            autoRotateAngle: [0, 45, 90],
          },
        },
      ],
      tooltip: {
        visible: true,
        mark: {
          title: {
            key: 'time',
            value: 'time',
          },
          content: [
            {
              key: (datum) => t('命中率'),
              value: (datum) => `${datum.cacheHitRate}%`,
            },
          ],
        },
      },
      color: {
        type: 'linear',
        x0: 0,
        y0: 0,
        x1: 1,
        y1: 0,
        stops: [
          { offset: 0, color: '#4096ff' },
          { offset: 1, color: '#52c41a' },
        ],
      },
    }),
    [chartDataPoints, t],
  );

  // Cost Saved Chart Spec
  const costSavedSpec = useMemo(
    () => ({
      type: 'area',
      data: [
        {
          id: 'costSaved',
          values: chartDataPoints,
        },
      ],
      xField: 'time',
      yField: 'costSaved',
      seriesField: 'type',
      line: {
        style: {
          lineWidth: 3,
          lineCap: 'round',
        },
      },
      area: {
        style: {
          fillOpacity: 0.3,
        },
      },
      point: {
        visible: true,
        style: {
          size: 5,
          fill: 'white',
          stroke: '#52c41a',
          lineWidth: 2,
        },
      },
      axes: [
        {
          orient: 'left',
          title: {
            visible: true,
            text: t('节省成本') + ` (${unitLabel})`,
          },
        },
        {
          orient: 'bottom',
          title: {
            visible: true,
            text: t('时间'),
          },
          label: {
            autoRotate: true,
            autoRotateAngle: [0, 45, 90],
          },
        },
      ],
      tooltip: {
        visible: true,
        mark: {
          title: {
            key: 'time',
            value: 'time',
          },
          content: [
            {
              key: (datum) => t('节省'),
              value: (datum) => `${datum.costSaved} ${unitLabel}`,
            },
          ],
        },
      },
      color: {
        type: 'linear',
        x0: 0,
        y0: 0,
        x1: 0,
        y1: 1,
        stops: [
          { offset: 0, color: '#52c41a' },
          { offset: 1, color: '#95de64' },
        ],
      },
    }),
    [chartDataPoints, unitLabel, t],
  );

  const hasData = chartDataPoints && chartDataPoints.length > 0;

  return (
    <Card
      className='!rounded-2xl shadow-sm'
      title={
        <div className='flex items-center gap-2'>
          <IconHistogram />
          {t('缓存趋势分析')}
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      <Tabs
        type='line'
        activeKey={activeChartTab}
        onChange={setActiveChartTab}
        style={{ padding: '0 20px' }}
      >
        <TabPane tab={<span>{t('缓存命中率')}</span>} itemKey='1' />
        <TabPane tab={<span>{t('成本节省')}</span>} itemKey='2' />
      </Tabs>

      <div className='h-96 p-4'>
        {!hasData ? (
          <div className='flex items-center justify-center h-full'>
            <Empty
              title={t('暂无数据')}
              description={t('选定时间范围内没有缓存数据')}
            />
          </div>
        ) : (
          <>
            {activeChartTab === '1' && (
              <VChart spec={hitRateSpec} option={CHART_CONFIG} />
            )}
            {activeChartTab === '2' && (
              <VChart spec={costSavedSpec} option={CHART_CONFIG} />
            )}
          </>
        )}
      </div>
    </Card>
  );
};

export default ChartsPanel;