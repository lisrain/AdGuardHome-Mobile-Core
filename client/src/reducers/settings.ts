import { handleActions } from 'redux-actions';

import * as actions from '../actions';

const settings = handleActions(
    {
        [actions.testUpstreamRequest.toString()]: (state: any) => ({
            ...state,
            processingTestUpstream: true,
        }),
        [actions.testUpstreamFailure.toString()]: (state: any) => ({
            ...state,
            processingTestUpstream: false,
        }),
        [actions.testUpstreamSuccess.toString()]: (state: any) => ({
            ...state,
            processingTestUpstream: false,
        }),
    },
    {
        processingTestUpstream: false,
    },
);

export default settings;
