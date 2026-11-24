import { createMemo } from 'solid-js';
import { statsState } from './stats';
import { dashboardState } from './dashboard';

/**
 * Derived state computations using createMemo.
 * These replace reselect-style selectors from the Redux setup.
 */

export const blockedPercentage = createMemo(() => {
    const { numDnsQueries, numBlockedFiltering } = statsState;
    if (!numDnsQueries) return 0;
    return Math.round((numBlockedFiltering / numDnsQueries) * 100);
});

export const isProtectionActive = createMemo(() => {
    return dashboardState.protectionEnabled && dashboardState.isCoreRunning;
});
