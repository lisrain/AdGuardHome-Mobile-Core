import { connect } from 'react-redux';

import { getStatsConfig, setStatsConfig, resetStats } from '../actions/stats';
import { clearLogs, getLogsConfig, setLogsConfig } from '../actions/queryLogs';

import Settings from '../components/Settings';

const mapStateToProps = (state: any) => {
    const { stats, queryLogs } = state;
    const props = {
        stats,
        queryLogs,
    };
    return props;
};

const mapDispatchToProps = {
    getStatsConfig,
    setStatsConfig,
    resetStats,
    clearLogs,
    getLogsConfig,
    setLogsConfig,
};

export default connect(mapStateToProps, mapDispatchToProps)(Settings);
