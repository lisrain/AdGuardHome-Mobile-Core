import { createMemo, Show, For } from 'solid-js';
import cn from 'clsx';
import copy from 'copy-to-clipboard';

import intl from 'panel/common/intl';
import type { Client, NormalizedTopClients } from 'panel/initialState';
import { sortIpOrString } from 'panel/helpers/helpers';
import { LOCAL_STORAGE_KEYS, LocalStorageHelper } from 'panel/helpers/localStorageHelper';
import { Table, type TableColumn } from 'panel/common/ui/Table';
import { Icon } from 'panel/common/ui/Icon';
import { Tooltip } from 'panel/common/ui/Tooltip';
import { addSuccessToast } from 'panel/stores/toasts';
import theme from 'panel/lib/theme';

import { TagCell } from './TagCell';

import { TableEmptyState } from '../TableEmptyState/TableEmptyState';
import s from './PersistentClientsTable.module.pcss';

type Props = {
    clients: Client[];
    normalizedTopClients?: NormalizedTopClients;
    loading?: boolean;
};

export const PersistentClientsTable = (props: Props) => {
    const pageSize = createMemo(
        () => LocalStorageHelper.getItem<number>(LOCAL_STORAGE_KEYS.CLIENTS_PAGE_SIZE) || undefined,
    );

    const handleCopy = (text: string) => {
        copy(text);
        addSuccessToast(intl.getMessage('copied'));
    };

    const columns = createMemo<TableColumn<Client>[]>(() => {
        return [
            {
                key: 'ids',
                header: {
                    text: intl.getMessage('client_identifier'),
                    className: s.headerCell,
                },
                // The cell shows only the first identifier, so sort by it.
                accessor: (row: Client) => (row.ids ?? []).find((id) => id.trim() !== '') ?? '',
                sortable: true,
                sortFn: sortIpOrString,
                render: (_value: string, row: Client) => {
                    const ids = row.ids ?? [];
                    // Filter out empty strings — the backend may return trailing empty
                    // entries that would inflate hiddenCount and cause a spurious comma.
                    const nonEmpty = ids.filter((id) => id.trim() !== '');
                    const firstId = nonEmpty[0] || '';
                    const hiddenCount = nonEmpty.length - 1;

                    return (
                        <div class={theme.table.cell}>
                            <span class={theme.table.cellLabel}>
                                {intl.getMessage('client_identifier')}
                            </span>

                            <div class={theme.table.cellValueText}>
                                <div class={s.idsRow}>
                                    <span
                                        class={cn(theme.common.textOverflow, s.idsText)}
                                        data-testid="client-ids-text"
                                    >
                                        {firstId}
                                    </span>
                                    <Show when={hiddenCount > 0}>
                                        <Tooltip
                                            class={s.countDropdown}
                                            overlayClass={s.idsTooltipOverlay}
                                            content={
                                                <div class={s.idsTooltip}>
                                                    <For each={nonEmpty}>
                                                        {(id) => (
                                                            <span class={s.idsTooltipItem}>
                                                                {id}
                                                            </span>
                                                        )}
                                                    </For>
                                                    <button
                                                        type="button"
                                                        class={cn(
                                                            s.copyBtn,
                                                            s.copyBtnGreen,
                                                            s.copyBtnTopRight,
                                                        )}
                                                        onClick={() =>
                                                            handleCopy(nonEmpty.join(', '))
                                                        }
                                                        title={intl.getMessage('copy')}
                                                    >
                                                        <Icon icon="copy" color="green" />
                                                    </button>
                                                </div>
                                            }
                                        >
                                            <span
                                                class={s.countLabel}
                                                data-testid="client-ids-count"
                                            >
                                                {hiddenCount}
                                            </span>
                                        </Tooltip>
                                    </Show>
                                </div>
                            </div>
                        </div>
                    );
                },
            },
            {
                key: 'name',
                header: {
                    text: intl.getMessage('name'),
                    className: s.headerCell,
                },
                accessor: 'name',
                sortable: true,
                render: (value: string) => (
                    <div class={theme.table.cell}>
                        <span class={theme.table.cellLabel}>{intl.getMessage('name')}</span>

                        <div class={theme.table.cellValueText}>
                            <Tooltip
                                overlayClass={s.nameTooltipOverlay}
                                content={
                                    <div class={s.nameTooltip}>
                                        <span class={s.nameTooltipText}>{value}</span>
                                        <button
                                            type="button"
                                            class={cn(s.copyBtn, s.copyBtnGreen)}
                                            onClick={() => handleCopy(value)}
                                            title={intl.getMessage('copy')}
                                        >
                                            <Icon icon="copy" color="green" />
                                        </button>
                                    </div>
                                }
                                class={s.nameDropdownInner}
                            >
                                <span class={cn(theme.common.twoRowsOverflow, s.nameTrigger)}>
                                    {value}
                                </span>
                            </Tooltip>
                        </div>
                    </div>
                ),
            },
            {
                key: 'tags',
                header: {
                    text: intl.getMessage('tags_title'),
                    className: s.headerCell,
                },
                accessor: (row: Client) => (row.tags ?? []).join(','),
                sortable: true,
                render: (_value: string, row: Client) => (
                    <TagCell tags={row.tags ?? []} onCopy={handleCopy} />
                ),
            },
            {
                key: 'requests',
                header: {
                    text: intl.getMessage('requests_table_header'),
                    className: s.headerCell,
                },
                accessor: (row: Client) =>
                    props.normalizedTopClients?.configured[row.name ?? ''] || 0,
                sortable: true,
                render: (_value: unknown, row: Client) => (
                    <div class={theme.table.cell}>
                        <span class={theme.table.cellLabel}>
                            {intl.getMessage('requests_table_header')}
                        </span>

                        <div class={theme.table.cellValueText}>
                            <span class={theme.common.textOverflow}>
                                {(
                                    props.normalizedTopClients?.configured[row.name ?? ''] || 0
                                ).toLocaleString()}
                            </span>
                        </div>
                    </div>
                ),
            },
        ];
    });

    const handlePageSizeChange = (newSize: number) => {
        LocalStorageHelper.setItem(LOCAL_STORAGE_KEYS.CLIENTS_PAGE_SIZE, newSize);
    };

    return (
        <Table<Client>
            data={props.clients}
            class={s.table}
            columns={columns()}
            emptyTable={<TableEmptyState />}
            loading={props.loading}
            pageSize={pageSize()}
            onPageSizeChange={handlePageSizeChange}
            getRowId={(row) => row.name ?? ''}
            headerTooltip
        />
    );
};
