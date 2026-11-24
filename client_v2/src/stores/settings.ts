import { createStore } from 'solid-js/store';
import { untrack } from 'solid-js';
import { testUpstreamDNS } from 'panel/api/generated';
import { addErrorToast, addSuccessToast } from './toasts';
import { splitByNewLine } from 'panel/helpers/helpers';
import intl from 'panel/common/intl';
import type { UpstreamsConfig } from 'panel/api/model/upstreamsConfig';

type SettingsState = {
    processingTestUpstream: boolean;
};

const initialState: SettingsState = {
    processingTestUpstream: false,
};

const [state, setState] = createStore<SettingsState>(initialState);

export const settingsState = untrack(() => state);

export const testUpstreamWithFormValues = async (
    formValues: {
        bootstrap_dns: string;
        upstream_dns: string;
        local_ptr_upstreams: string;
        fallback_dns: string;
    },
    upstreamDnsFile?: string,
) => {
    setState('processingTestUpstream', true);
    try {
        const { bootstrap_dns, upstream_dns, local_ptr_upstreams, fallback_dns } = formValues;

        const filterOutComments = (lines: string[]) =>
            lines.filter((line) => !line.startsWith('#') && !line.startsWith('!'));
        const removeComments = (text: string) => filterOutComments(splitByNewLine(text));

        const config: UpstreamsConfig = {
            bootstrap_dns: splitByNewLine(bootstrap_dns),
            private_upstream: splitByNewLine(local_ptr_upstreams),
            fallback_dns: splitByNewLine(fallback_dns),
            ...(upstreamDnsFile ? null : { upstream_dns: removeComments(upstream_dns) }),
        };

        const upstreamResponse = await testUpstreamDNS(config);
        const testMessages = Object.keys(upstreamResponse).map((key) => {
            const message = upstreamResponse[key];
            if (message.startsWith('WARNING:')) {
                addErrorToast({
                    error: intl.getMessage('dns_test_warning_toast', { key }),
                });
            } else if (message.endsWith(': parsing error')) {
                const info = message.substring(0, message.indexOf(':'));
                const [sectionKey, line] = info.split(' ');
                addErrorToast({
                    error: intl.getMessage('dns_test_parsing_error_toast', {
                        section: sectionKey,
                        number: line,
                    }),
                });
            } else if (message !== 'OK') {
                addErrorToast({ error: intl.getMessage('dns_test_not_ok_toast', { key }) });
            }
            return message;
        });

        if (testMessages.every((message) => message === 'OK' || message.startsWith('WARNING:'))) {
            addSuccessToast(intl.getMessage('dns_test_ok_toast'));
        }
    } catch (error) {
        addErrorToast({ error });
    } finally {
        setState('processingTestUpstream', false);
    }
};
