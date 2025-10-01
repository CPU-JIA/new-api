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

import { API } from './api';

/**
 * Get cache metrics overview summary
 * @param {string} period - Time period: '1h', '24h', '7d', '30d'
 * @returns {Promise} API response with overview data
 */
export async function getCacheOverview(period = '24h') {
  const res = await API.get(`/api/cache/metrics/overview?period=${period}`);
  return res.data;
}

/**
 * Get cache metrics time-series data for charts
 * @param {string} period - Time period: '1h', '24h', '7d', '30d'
 * @param {string} interval - Data interval: '1m', '5m', '15m', '1h', '1d'
 * @returns {Promise} API response with chart data
 */
export async function getCacheChartData(period = '24h', interval = '1h') {
  const res = await API.get(`/api/cache/metrics/chart?period=${period}&interval=${interval}`);
  return res.data;
}

/**
 * Get cache metrics grouped by channels (admin only)
 * @param {string} period - Time period: '1h', '24h', '7d', '30d'
 * @returns {Promise} API response with channel-grouped metrics
 */
export async function getCacheChannels(period = '24h') {
  const res = await API.get(`/api/cache/metrics/channels?period=${period}`);
  return res.data;
}

/**
 * Get cache metrics for a specific user
 * @param {number} userId - User ID
 * @param {string} period - Time period: '1h', '24h', '7d', '30d'
 * @returns {Promise} API response with user metrics
 */
export async function getCacheUserMetrics(userId, period = '24h') {
  const res = await API.get(`/api/cache/metrics/user/${userId}?period=${period}`);
  return res.data;
}

/**
 * Get real-time CacheWarmer service status (admin only)
 * @returns {Promise} API response with warmup status
 */
export async function getWarmupStatus() {
  const res = await API.get('/api/cache/warmer/status');
  return res.data;
}