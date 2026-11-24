import { RootState } from 'panel/initialState';

import { CheckResultData, RewriteEntry } from './types';

export const getPrimaryRule = (result?: CheckResultData | null) => result?.rules?.[0];

const matchesDomain = (hostname: string, pattern: string): boolean => {
    const h = hostname.toLowerCase();
    const p = pattern.toLowerCase();

    if (p === h) {
        return true;
    }

    return p.startsWith('*.') && h.endsWith(p.slice(1));
};

export const findMatchedRewrite = (
    rewrites: RootState['rewrites']['list'],
    checkResult?: CheckResultData | null,
): RewriteEntry | null => {
    if (!checkResult?.hostname) {
        return null;
    }

    const matches = rewrites.filter((entry) => matchesDomain(checkResult.hostname!, entry.domain));

    return matches.length === 1 ? matches[0] : null;
};
