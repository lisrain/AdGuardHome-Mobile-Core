import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import userEvent from '@testing-library/user-event';
import { HashRouter, Route } from '@solidjs/router';

// We'll mock the stores at the module level and update them per test
const mockDashboardState = {
    isUpdateAvailable: false,
    newVersion: '',
    announcementUrl: '',
    canAutoUpdate: false,
    processingUpdate: false,
    processingVersion: false,
    dnsVersion: '',
};

vi.mock('panel/stores/dashboard', () => ({
    get dashboardState() {
        return mockDashboardState;
    },
    getUpdate: vi.fn(),
    getVersion: vi.fn(),
}));

// The banners render localized copy; serve it from the base locale so the
// assertions can name the key instead of re-stating the text.
vi.mock('panel/common/intl', async () =>
    (await import('panel/__tests__/helpers/copy')).createIntlMock(),
);

import { Banners } from 'panel/common/ui/Banners';
import { BANNER_TEST_VALUES } from 'panel/helpers/banners';

const renderBanners = (props?: { forceBanner?: any }) => {
    return render(() => (
        <HashRouter>
            <Route path="/" component={() => <Banners {...props} />} />
        </HashRouter>
    ));
};

const resetStores = () => {
    mockDashboardState.isUpdateAvailable = false;
    mockDashboardState.newVersion = '';
    mockDashboardState.announcementUrl = '';
    mockDashboardState.canAutoUpdate = false;
    mockDashboardState.processingUpdate = false;
    mockDashboardState.processingVersion = false;
    mockDashboardState.dnsVersion = '';
};

describe('Banners', () => {
    beforeEach(() => {
        resetStores();
        // The Update buttons navigate; start every test from the root route so
        // a previous navigation cannot hide the banner.
        window.location.hash = '';
    });

    it('shows auto-update banner when update available and can auto-update', () => {
        mockDashboardState.isUpdateAvailable = true;
        mockDashboardState.newVersion = 'v1.0.0';
        mockDashboardState.canAutoUpdate = true;

        renderBanners();

        expect(screen.getByTestId('banner-update-auto')).toBeInTheDocument();
    });

    it('shows manual-update banner when update available but cannot auto-update', () => {
        mockDashboardState.isUpdateAvailable = true;
        mockDashboardState.newVersion = 'v1.0.0';
        mockDashboardState.canAutoUpdate = false;

        renderBanners();

        expect(screen.getByTestId('banner-update-manual')).toBeInTheDocument();
    });

    it('renders nothing when no conditions met', () => {
        renderBanners();

        expect(screen.queryByTestId('banner-root')).not.toBeInTheDocument();
    });

    it('hides the banner after clicking the close button', async () => {
        const user = userEvent.setup();
        mockDashboardState.isUpdateAvailable = true;
        mockDashboardState.newVersion = 'v1.0.0';
        mockDashboardState.canAutoUpdate = true;

        renderBanners();

        expect(screen.getByTestId('banner-update-auto')).toBeInTheDocument();
        await user.click(screen.getByTestId('banner-update-auto-close'));

        expect(screen.queryByTestId('banner-root')).not.toBeInTheDocument();
    });

    it('renders forced auto-update banner', () => {
        renderBanners({ forceBanner: BANNER_TEST_VALUES.updateAuto });
        expect(screen.getByTestId('banner-update-auto')).toBeInTheDocument();
    });

    it('renders forced manual-update banner', () => {
        renderBanners({ forceBanner: BANNER_TEST_VALUES.updateManual });
        expect(screen.getByTestId('banner-update-manual')).toBeInTheDocument();
    });

    it('forceBanner overrides real banner conditions', () => {
        mockDashboardState.isUpdateAvailable = true;
        mockDashboardState.newVersion = 'v1.0.0';
        mockDashboardState.canAutoUpdate = false;

        renderBanners({ forceBanner: BANNER_TEST_VALUES.updateAuto });

        expect(screen.getByTestId('banner-update-auto')).toBeInTheDocument();
        expect(screen.queryByTestId('banner-update-manual')).not.toBeInTheDocument();
    });
});
