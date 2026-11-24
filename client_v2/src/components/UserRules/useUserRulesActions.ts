import { createSignal, createMemo, untrack } from 'solid-js';

import intl from 'panel/common/intl';
import {
    filteringState,
    checkHost,
    getFilteringStatus,
    setRules,
    toggleFilterStatus,
    toggleBlocking,
    toggleBlockingForClient,
    BLOCK_ACTIONS,
} from 'panel/stores/filtering';
import { updateRewrite, deleteRewrite, addRewrite, getRewritesList } from 'panel/stores/rewrites';
import { addSuccessToast, createUndoToast } from 'panel/stores/toasts';
import { openModal } from 'panel/stores/modals';
import { MODAL_TYPE, SPECIAL_FILTER_ID } from 'panel/helpers/constants';
import { delay, splitByNewLine } from 'panel/helpers/helpers';

import { findMatchedRewrite, getPrimaryRule } from './helpers';
import {
    type CheckFormValues,
    type CheckResultData,
    type ResultActionKind,
    type RewriteEntry,
} from './types';

const EMPTY_REWRITE: RewriteEntry = {
    domain: '',
    answer: '',
    enabled: false,
};

const RECHECK_DELAY_MS = 500;

type UseUserRulesActionsParams = {
    checkResult: () => CheckResultData | null;
    rewritesList: () => any;
    lastSubmittedCheck: () => CheckFormValues | null;
    setIsResultVisible: (visible: boolean) => void;
};

type UseUserRulesActionsResult = {
    currentRewrite: () => RewriteEntry;
    setCurrentRewrite: (value: RewriteEntry | ((prev: RewriteEntry) => RewriteEntry)) => void;
    matchedRewrite: () => RewriteEntry | null;
    handleAction: (action: ResultActionKind) => Promise<void>;
    handleRewriteUpdate: (update: RewriteEntry) => Promise<boolean>;
    handleRewriteDelete: () => Promise<boolean>;
    openEditRewriteModal: () => void;
    openDeleteRewriteModal: () => void;
    resetCurrentRewrite: () => void;
};

export const useUserRulesActions = (
    params: UseUserRulesActionsParams,
): UseUserRulesActionsResult => {
    const [currentRewrite, setCurrentRewrite] = createSignal<RewriteEntry>(EMPTY_REWRITE);

    const matchedRewrite = createMemo(() =>
        findMatchedRewrite(params.rewritesList(), params.checkResult()),
    );

    const runWithClosedResult = async <T>(callback: () => Promise<T>): Promise<T> => {
        params.setIsResultVisible(false);
        return callback();
    };

    const recheckCurrentTarget = async () => {
        const lastCheck = params.lastSubmittedCheck();
        if (!lastCheck) {
            return;
        }

        await delay(RECHECK_DELAY_MS);

        await checkHost({
            name: lastCheck.hostname,
            client: lastCheck.client || undefined,
            qtype: lastCheck.qtype || undefined,
        });

        params.setIsResultVisible(true);
    };

    const performWithUndo = async (p: {
        perform: () => Promise<boolean | undefined | void>;
        message: string;
        undo: () => Promise<boolean | undefined | void>;
        refresh: () => Promise<void>;
    }) => {
        return runWithClosedResult(async () => {
            const ok = await p.perform();

            if (ok === false) {
                return false;
            }

            addSuccessToast(
                createUndoToast(p.message, intl.getMessage('notify_undo'), async () => {
                    const didUndo = await p.undo();
                    if (didUndo) {
                        await p.refresh();
                    }
                }),
            );
            await recheckCurrentTarget();

            return true;
        });
    };

    const handleRuleToggle = async (type: (typeof BLOCK_ACTIONS)[keyof typeof BLOCK_ACTIONS]) => {
        const result = params.checkResult();
        const lastCheck = params.lastSubmittedCheck();
        if (!result?.hostname || !lastCheck) {
            return;
        }

        const primaryRule = getPrimaryRule(result);
        const matchedCustomRule =
            !lastCheck.client &&
            primaryRule?.filter_list_id === SPECIAL_FILTER_ID.CUSTOM_FILTERING_RULES
                ? primaryRule.text
                : undefined;

        await runWithClosedResult(async () => {
            let ok: boolean | undefined;
            if (lastCheck.client) {
                ok = await toggleBlockingForClient(type, result.hostname, lastCheck.client);
            } else if (matchedCustomRule) {
                ok = await toggleBlocking(
                    type,
                    result.hostname,
                    undefined,
                    undefined,
                    matchedCustomRule,
                );
            } else {
                ok = await toggleBlocking(type, result.hostname);
            }

            if (ok === false) {
                return false;
            }

            await recheckCurrentTarget();
            return true;
        });
    };

    const handleDisableFilter = async () => {
        const result = params.checkResult();
        const filterId = getPrimaryRule(result)?.filter_list_id;
        if (filterId === undefined) return;

        const filters = filteringState.filters;
        const whitelistFilters = filteringState.whitelistFilters;

        const matchedFilter =
            filters.find((filter: any) => filter.id === filterId) ??
            whitelistFilters.find((filter: any) => filter.id === filterId);
        if (!matchedFilter) return;

        const isWhitelist = whitelistFilters.some((filter: any) => filter.id === filterId);

        await performWithUndo({
            perform: () =>
                toggleFilterStatus(
                    matchedFilter.url,
                    { name: matchedFilter.name, url: matchedFilter.url, enabled: false },
                    isWhitelist,
                ),
            message: intl.getMessage('user_rules_filter_was_disabled', {
                value: matchedFilter.name,
            }),
            undo: () =>
                toggleFilterStatus(
                    matchedFilter.url,
                    { name: matchedFilter.name, url: matchedFilter.url, enabled: true },
                    isWhitelist,
                ),
            refresh: () => recheckCurrentTarget(),
        });
    };

    const handleRewriteUpdate = async (update: RewriteEntry) => {
        const rewrite = untrack(() => currentRewrite());
        if (!rewrite.domain) {
            return false;
        }

        return runWithClosedResult(async () => {
            const ok = await updateRewrite(
                { target: untrack(() => currentRewrite()), update },
                { showToast: false, closeModal: false },
            );

            if (!ok) {
                return false;
            }

            addSuccessToast(intl.getMessage('settings_notify_changes_saved'));
            await recheckCurrentTarget();

            return true;
        });
    };

    const handleRewriteDelete = async (rewrite?: RewriteEntry) => {
        const targetRewrite = rewrite || currentRewrite();

        if (!targetRewrite.domain) {
            return false;
        }

        return performWithUndo({
            perform: () => deleteRewrite(targetRewrite) as Promise<boolean>,
            message: intl.getMessage('user_rules_dns_rewrite_removed'),
            undo: async () => {
                await addRewrite({
                    domain: targetRewrite.domain,
                    answer: targetRewrite.answer,
                    enabled: targetRewrite.enabled,
                });
                return true;
            },
            refresh: async () => {
                await getRewritesList();
            },
        });
    };

    const resetCurrentRewrite = () => {
        setCurrentRewrite(EMPTY_REWRITE);
    };

    const openEditRewriteModal = () => {
        const matched = matchedRewrite();
        if (!matched) return;
        setCurrentRewrite(matched);
        openModal(MODAL_TYPE.EDIT_REWRITE);
    };

    const openDeleteRewriteModal = () => {
        const matched = matchedRewrite();
        if (!matched) return;
        handleRewriteDelete(matched);
    };

    const handleRemoveRewriteRule = async () => {
        const result = params.checkResult();
        const primaryRule = getPrimaryRule(result);

        if (!primaryRule?.text) return;

        const ruleToRemove = primaryRule.text;
        const originalRules = filteringState.userRules || '';

        await performWithUndo({
            perform: () => {
                const currentRules = splitByNewLine(filteringState.userRules || '');
                const updatedRules = currentRules.filter((rule: string) => rule !== ruleToRemove);
                return setRules(`${updatedRules.join('\n')}\n`);
            },
            message: intl.getMessage('user_rules_rule_removed'),
            undo: () => setRules(originalRules),
            refresh: async () => {
                await getFilteringStatus();
            },
        });
    };

    const handleAction = async (action: ResultActionKind) => {
        switch (action) {
            case 'allow':
                await handleRuleToggle(BLOCK_ACTIONS.UNBLOCK);
                break;
            case 'block':
                await handleRuleToggle(BLOCK_ACTIONS.BLOCK);
                break;
            case 'disable-filter':
                await handleDisableFilter();
                break;
            case 'remove-rewrite-rule':
                await handleRemoveRewriteRule();
                break;
            case 'edit-rewrite':
            case 'delete-rewrite':
            default:
                break;
        }
    };

    return {
        currentRewrite,
        setCurrentRewrite,
        matchedRewrite,
        handleAction,
        handleRewriteUpdate,
        handleRewriteDelete,
        openEditRewriteModal,
        openDeleteRewriteModal,
        resetCurrentRewrite,
    };
};
