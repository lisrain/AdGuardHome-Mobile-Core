import { handleActions } from 'redux-actions';

import { DAY, HOUR, STATS_INTERVALS_DAYS, TIME_UNITS } from '../helpers/constants';

import * as actions from '../actions/stats';

const defaultStats = {
    dnsQueries: [],
    blockedFiltering: [],
    topBlockedDomains: [],
    topQueriedDomains: [],
    numBlockedFiltering: 0,
    numDnsQueries: 0,
    avgProcessingTime: 0,
    timeUnits: TIME_UNITS.HOURS,
};

const stats = handleActions(
    {
        [actions.getStatsConfigRequest.toString()]: (state: any) => ({
            ...state,
            processingGetConfig: true,
        }),
        [actions.getStatsConfigFailure.toString()]: (state: any) => ({
            ...state,
            processingGetConfig: false,
        }),
        [actions.getStatsConfigSuccess.toString()]: (state, { payload }: any) => ({
            ...state,
            ...payload,

            customInterval: !STATS_INTERVALS_DAYS.includes(payload.interval) ? payload.interval / HOUR : null,

            processingGetConfig: false,
        }),

        [actions.setStatsConfigRequest.toString()]: (state: any) => ({
            ...state,
            processingSetConfig: true,
        }),
        [actions.setStatsConfigFailure.toString()]: (state: any) => ({
            ...state,
            processingSetConfig: false,
        }),
        [actions.setStatsConfigSuccess.toString()]: (state, { payload }: any) => ({
            ...state,
            ...payload,
            processingSetConfig: false,
        }),

        [actions.getStatsRequest.toString()]: (state: any) => ({
            ...state,
            processingStats: true,
        }),
        [actions.getStatsFailure.toString()]: (state: any) => ({
            ...state,
            processingStats: false,
        }),
        [actions.getStatsSuccess.toString()]: (state: any, { payload }: any) => {
            const {
                dns_queries: dnsQueries,
                blocked_filtering: blockedFiltering,
                top_blocked_domains: topBlockedDomains,
                top_queried_domains: topQueriedDomains,
                num_blocked_filtering: numBlockedFiltering,
                num_dns_queries: numDnsQueries,
                avg_processing_time: avgProcessingTime,
                top_upstreams_responses: topUpstreamsResponses,
                top_upstrems_avg_time: topUpstreamsAvgTime,
                time_units: timeUnits,
            } = payload;

            const newState = {
                ...state,
                processingStats: false,
                dnsQueries,
                blockedFiltering,
                topBlockedDomains,
                topQueriedDomains,
                numBlockedFiltering,
                numDnsQueries,
                avgProcessingTime,
                topUpstreamsResponses,
                topUpstreamsAvgTime,
                timeUnits,
            };

            return newState;
        },

        [actions.resetStatsRequest.toString()]: (state: any) => ({
            ...state,
            processingReset: true,
        }),
        [actions.resetStatsFailure.toString()]: (state: any) => ({
            ...state,
            processingReset: false,
        }),
        [actions.resetStatsSuccess.toString()]: (state: any) => ({
            ...state,
            ...defaultStats,
            processingReset: false,
        }),
    },
    {
        processingGetConfig: false,
        processingSetConfig: false,
        processingStats: true,
        processingReset: false,
        interval: DAY,
        customInterval: null,
        ignored_enabled: true,
        ...defaultStats,
    },
);

export default stats;
