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
import { Card, Tag, Typography } from '@douyinfe/semi-ui';
import {
  IconTick,
  IconCommand,
  IconCoinMoneyStroked,
  IconHistogram,
} from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const STATUS_COLORS = {
  success: 'green',
  warning: 'amber',
  danger: 'red',
};

const ICON_MAP = {
  0: IconCommand,
  1: IconHistogram,
  2: IconCoinMoneyStroked,
  3: IconTick,
};

const StatsCards = ({ statsCards }) => {
  return (
    <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4'>
      {statsCards.map((card, idx) => {
        const Icon = ICON_MAP[idx] || IconCommand;
        const statusColor = card.status ? STATUS_COLORS[card.status] : null;

        return (
          <Card
            key={idx}
            className='border-0 !rounded-2xl shadow-sm hover:shadow-md transition-shadow'
            bodyStyle={{ padding: '20px' }}
          >
            <div className='flex items-start justify-between mb-3'>
              <div className='flex items-center'>
                <div
                  className={`w-10 h-10 rounded-lg flex items-center justify-center ${
                    statusColor
                      ? `bg-${statusColor}-100`
                      : 'bg-blue-100'
                  }`}
                >
                  <Icon
                    size='large'
                    className={statusColor ? `text-${statusColor}-600` : 'text-blue-600'}
                  />
                </div>
              </div>
              {statusColor && (
                <Tag
                  color={statusColor}
                  size='small'
                  shape='circle'
                  style={{ marginTop: '2px' }}
                >
                  {card.status}
                </Tag>
              )}
            </div>

            <div className='mb-2'>
              <Text type='tertiary' size='small'>
                {card.title}
              </Text>
            </div>

            <div className='flex items-baseline mb-2'>
              <Title heading={2} style={{ margin: 0 }}>
                {card.value}
              </Title>
              {card.unit && (
                <Text type='secondary' className='ml-1'>
                  {card.unit}
                </Text>
              )}
            </div>

            <div>
              <Text type='tertiary' size='small'>
                {card.description}
              </Text>
              {card.extra && (
                <div className='mt-1'>
                  <Text type='tertiary' size='small'>
                    {card.extra}
                  </Text>
                </div>
              )}
            </div>
          </Card>
        );
      })}
    </div>
  );
};

export default StatsCards;