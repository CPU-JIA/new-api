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

import { useState, useCallback, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  getCacheOverview,
  getCacheChartData,
  getCacheChannels,
  getWarmupStatus,
} from '../../helpers/cacheApi';
import { isAdmin, showError, showSuccess } from '../../helpers';
import { useMinimumLoadingTime } from '../common/useMinimumLoadingTime';

const PERIOD_OPTIONS = [
  { label: '最近1小时', value: '1h' },
  { label: '最近24小时', value: '24h' },
  { label: '最近7天', value: '7d' },
  { label: '最近30天', value: '30d' },
];

const INTERVAL_MAP = {
  '1h': '1m',
  '24h': '1h',
  '7d': '1h',
  '30d': '1d',
};

export const useCacheMetrics = (userState) => {
  const { t } = useTranslation();
  const isAdminUser = isAdmin();

  // ========== 基础状态 ==========
  const [loading, setLoading] = useState(false);
  const showLoading = useMinimumLoadingTime(loading);
  const [period, setPeriod] = useState('24h');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(30000); // 30s default

  // ========== 🔥 成本单位状态 ==========
  const [costUnit, setCostUnit] = useState(() => {
    // 从localStorage恢复用户偏好
    return localStorage.getItem('cache_cost_unit') || 'quota';
  });

  // ========== 数据状态 ==========
  const [overviewData, setOverviewData] = useState(null);
  const [chartData, setChartData] = useState(null);
  const [channelMetrics, setChannelMetrics] = useState([]);
  const [warmupStatus, setWarmupStatus] = useState(null);

  // ========== 活跃Tab状态 ==========
  const [activeChartTab, setActiveChartTab] = useState('1'); // 1: Cache Hit Rate, 2: Cost Saved

  // ========== Memoized Values ==========
  const periodOptions = useMemo(
    () =>
      PERIOD_OPTIONS.map((option) => ({
        ...option,
        label: t(option.label),
      })),
    [t],
  );

  const interval = useMemo(() => INTERVAL_MAP[period] || '1h', [period]);

  // 🔥 Get unit label and description
  const unitLabel = useMemo(() => {
    switch (costUnit) {
      case 'usd':
        return 'USD ($)';
      case 'cny':
        return 'CNY (¥)';
      case 'tokens':
        return 'Tokens';
      case 'quota':
      default:
        return '额度';
    }
  }, [costUnit]);

  const statsCards = useMemo(() => {
    if (!overviewData?.data) {
      return [
        { title: '总请求数', value: '0', unit: '', description: '缓存相关请求' },
        {
          title: '缓存命中率',
          value: '0.00',
          unit: '%',
          description: '平均命中率',
        },
        {
          title: '总节省成本',
          value: '0.00',
          unit: '',
          description: '缓存节省额度',
        },
        {
          title: '净节省',
          value: '0.00',
          unit: '',
          description: '扣除Warmup成本',
        },
      ];
    }

    const {
      total_requests,
      cache_hit_rate,
      active_warmup_channels,
      cost_saved_quota,
      cost_saved_usd,
      cost_saved_cny,
      cost_saved_tokens,
      net_savings_quota,
      net_savings_usd,
      net_savings_cny,
      net_savings_tokens,
    } = overviewData.data;

    // 🔥 根据用户选择的单位格式化显示值
    const getCostDisplay = (quotaValue, usdValue, cnyValue, tokensValue) => {
      switch (costUnit) {
        case 'usd':
          return {
            value: (usdValue || 0).toFixed(6),
            unit: 'USD',
            prefix: '$',
          };
        case 'cny':
          return {
            value: (cnyValue || 0).toFixed(4),
            unit: 'CNY',
            prefix: '¥',
          };
        case 'tokens':
          return {
            value: (tokensValue || 0).toLocaleString(),
            unit: 'Tokens',
            prefix: '',
          };
        case 'quota':
        default:
          return {
            value: (quotaValue || 0).toFixed(2),
            unit: '额度',
            prefix: '',
          };
      }
    };

    const costSavedDisplay = getCostDisplay(
      cost_saved_quota,
      cost_saved_usd,
      cost_saved_cny,
      cost_saved_tokens,
    );
    const netSavingsDisplay = getCostDisplay(
      net_savings_quota,
      net_savings_usd,
      net_savings_cny,
      net_savings_tokens,
    );

    return [
      {
        title: '总请求数',
        value: (total_requests || 0).toLocaleString(),
        unit: '',
        description: '缓存相关请求',
        extra: `活跃Warmup渠道: ${active_warmup_channels || 0}`,
      },
      {
        title: '缓存命中率',
        value: ((cache_hit_rate || 0) * 100).toFixed(2),
        unit: '%',
        description: '平均命中率',
        status:
          cache_hit_rate >= 0.8 ? 'success' : cache_hit_rate >= 0.5 ? 'warning' : 'danger',
      },
      {
        title: '总节省成本',
        value: costSavedDisplay.prefix + costSavedDisplay.value,
        unit: costSavedDisplay.unit,
        description: `缓存节省的${costSavedDisplay.unit}`,
      },
      {
        title: '净节省',
        value: netSavingsDisplay.prefix + netSavingsDisplay.value,
        unit: netSavingsDisplay.unit,
        description: '扣除Warmup成本后',
        status: (net_savings_quota || 0) > 0 ? 'success' : 'warning',
      },
    ];
  }, [overviewData, costUnit]);

  // ========== API 调用函数 ==========
  const loadOverviewData = useCallback(async () => {
    try {
      const res = await getCacheOverview(period);
      if (res.success) {
        setOverviewData(res);
      } else {
        showError(res.message || '获取概览数据失败');
      }
    } catch (err) {
      console.error('Failed to load cache overview:', err);
      showError('加载概览数据时发生错误');
    }
  }, [period]);

  const loadChartData = useCallback(async () => {
    try {
      const res = await getCacheChartData(period, interval);
      if (res.success) {
        setChartData(res.data);
      } else {
        showError(res.message || '获取图表数据失败');
      }
    } catch (err) {
      console.error('Failed to load chart data:', err);
      showError('加载图表数据时发生错误');
    }
  }, [period, interval]);

  const loadChannelMetrics = useCallback(async () => {
    if (!isAdminUser) return;

    try {
      const res = await getCacheChannels(period);
      if (res.success) {
        setChannelMetrics(res.data || []);
      } else {
        showError(res.message || '获取渠道数据失败');
      }
    } catch (err) {
      console.error('Failed to load channel metrics:', err);
      showError('加载渠道数据时发生错误');
    }
  }, [period, isAdminUser]);

  const loadWarmupStatus = useCallback(async () => {
    if (!isAdminUser) return;

    try {
      const res = await getWarmupStatus();
      if (res.success) {
        setWarmupStatus(res.data);
      } else {
        showError(res.message || '获取Warmup状态失败');
      }
    } catch (err) {
      console.error('Failed to load warmup status:', err);
      showError('加载Warmup状态时发生错误');
    }
  }, [isAdminUser]);

  // 纯数据获取函数（不显示成功Toast，用于初始加载和自动刷新）
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      await Promise.all([
        loadOverviewData(),
        loadChartData(),
        loadChannelMetrics(),
        loadWarmupStatus(),
      ]);
    } catch (err) {
      console.error('Failed to fetch data:', err);
      showError('获取数据失败');
    } finally {
      setLoading(false);
    }
  }, [loadOverviewData, loadChartData, loadChannelMetrics, loadWarmupStatus]);

  // 手动刷新函数（显示成功Toast，仅用于用户点击刷新按钮）
  const refresh = useCallback(async () => {
    await fetchData();
    showSuccess('数据刷新成功');
  }, [fetchData]);

  // ========== 回调函数 ==========
  const handlePeriodChange = useCallback((value) => {
    setPeriod(value);
  }, []);

  const handleAutoRefreshToggle = useCallback((checked) => {
    setAutoRefresh(checked);
  }, []);

  const handleRefreshIntervalChange = useCallback((value) => {
    setRefreshInterval(value * 1000); // Convert to milliseconds
  }, []);

  // 🔥 Handle cost unit change
  const handleCostUnitChange = useCallback((value) => {
    setCostUnit(value);
    localStorage.setItem('cache_cost_unit', value); // 持久化用户偏好
  }, []);

  // ========== Effects ==========
  // Initial load and period change (silent, no toast)
  useEffect(() => {
    fetchData();
  }, [period]); // Fetch data when period changes

  // Auto-refresh (silent, no toast)
  useEffect(() => {
    if (!autoRefresh) return;

    const timer = setInterval(() => {
      fetchData();
    }, refreshInterval);

    return () => clearInterval(timer);
  }, [autoRefresh, refreshInterval, fetchData]);

  return {
    // 基础状态
    loading: showLoading,
    period,
    periodOptions,
    autoRefresh,
    refreshInterval,
    isAdminUser,

    // 🔥 成本单位状态
    costUnit,
    unitLabel,
    handleCostUnitChange,

    // 数据状态
    overviewData,
    chartData,
    channelMetrics,
    warmupStatus,
    statsCards,

    // 图表状态
    activeChartTab,
    setActiveChartTab,
    interval,

    // 函数
    handlePeriodChange,
    handleAutoRefreshToggle,
    handleRefreshIntervalChange,
    refresh,

    // 翻译
    t,
  };
};