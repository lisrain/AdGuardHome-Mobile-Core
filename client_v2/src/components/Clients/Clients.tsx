import { createMemo, onMount } from 'solid-js';
import { useSearchParams } from '@solidjs/router';
import cn from 'clsx';

import intl from 'panel/common/intl';
import { Tabs } from 'panel/common/ui/Tabs';
import { dashboardState, getClients } from 'panel/stores/dashboard';
import { statsState, getStats } from 'panel/stores/stats';
import theme from 'panel/lib/theme';

import { PersistentClientsTable } from './blocks/PersistentClientsTable';
import { RuntimeClientsTable } from './blocks/RuntimeClientsTable';
import s from './Clients.module.pcss';

const CLIENT_TABS = {
    PERSISTENT: 'persistent',
    RUNTIME: 'runtime',
} as const;

export const Clients = () => {
    const [searchParams, setSearchParams] = useSearchParams<{ tab?: string }>();

    const activeTab = createMemo(() =>
        searchParams.tab === CLIENT_TABS.RUNTIME ? CLIENT_TABS.RUNTIME : CLIENT_TABS.PERSISTENT,
    );

    const handleTabChange = (tabId: string) => {
        setSearchParams({ tab: tabId }, { replace: true });
    };

    onMount(() => {
        getClients();
        getStats();
    });

    // The tables show a loader only until their first request settles; later
    // refetches keep the previous rows painted.
    const isLoading = createMemo(
        () => !dashboardState.clientsInitialized || !statsState.statsInitialized,
    );

    return (
        <div class={theme.layout.container}>
            <div class={theme.layout.containerIn}>
                <h1
                    class={cn(theme.layout.title, theme.title.h4, theme.title.h3_tablet)}
                    data-testid="clients-title"
                >
                    {intl.getMessage('clients')}
                </h1>

                <Tabs
                    activeTab={activeTab()}
                    onTabChange={handleTabChange}
                    class={s.tabs}
                    variant="filled"
                    fullWidth
                    contentClass={s.tabContent}
                    tabs={[
                        {
                            id: CLIENT_TABS.PERSISTENT,
                            label: intl.getMessage('clients_title'),
                            content: (
                                <>
                                    <div class={s.desc}>{intl.getMessage('clients_desc')}</div>

                                    {dashboardState.clients?.length > 0 && (
                                        <div class={s.tableSection}>
                                            <PersistentClientsTable
                                                clients={dashboardState.clients || []}
                                                normalizedTopClients={
                                                    statsState.normalizedTopClients
                                                }
                                                loading={isLoading()}
                                            />
                                        </div>
                                    )}
                                </>
                            ),
                        },
                        {
                            id: CLIENT_TABS.RUNTIME,
                            label: intl.getMessage('auto_clients_title'),
                            content: (
                                <>
                                    <div class={s.desc}>{intl.getMessage('auto_clients_desc')}</div>

                                    {dashboardState.autoClients?.length > 0 && (
                                        <div class={s.tableSection}>
                                            <RuntimeClientsTable
                                                autoClients={dashboardState.autoClients || []}
                                                normalizedTopClients={
                                                    statsState.normalizedTopClients
                                                }
                                                loading={isLoading()}
                                            />
                                        </div>
                                    )}
                                </>
                            ),
                        },
                    ]}
                />
            </div>
        </div>
    );
};
