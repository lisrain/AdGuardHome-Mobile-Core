import { connect } from 'react-redux';
import { getDnsConfig, setDnsConfig } from '../actions/dnsConfig';

import Dns from '../components/Settings/Dns';

const mapStateToProps = (state: any) => {
    const { dnsConfig } = state;
    const props = {
        dnsConfig,
    };
    return props;
};

const mapDispatchToProps = {
    getDnsConfig,
    setDnsConfig,
};

export default connect(mapStateToProps, mapDispatchToProps)(Dns);
