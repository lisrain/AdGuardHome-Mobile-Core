import { type JSX, createSignal, createMemo, Show } from 'solid-js';
import intl from 'panel/common/intl';
import cn from 'clsx';

import { Select } from 'panel/common/controls/Select';
import { type IconType } from '../Icons';
import { CopiedText } from '../CopiedText';
import s from './Guide.module.pcss';

type PlatformLayout = {
    title: string;
    icon: IconType;
    component: JSX.Element;
};

type PlatformLayouts = Record<string, PlatformLayout>;

const RouterLayout = () => (
    <div class={s.guideContent}>
        <div class={s.title}>{intl.getMessage('setup_devices_router_title')}</div>
        <div class={s.guideText}>
            <div class={s.guideParagraph}>{intl.getMessage('setup_devices_router_desc_1')}</div>
            <ol class={cn({ [s.strongNumbers]: true })}>
                <li>
                    <strong class={s.guideItemTitle}>
                        {intl.getMessage('setup_devices_router_list_1_title')}
                    </strong>
                    {intl.getMessage('setup_devices_router_list_1', {
                        code: () => <CopiedText text="http://192.168.0.1" />,
                        code2: () => <CopiedText text="http://192.168.1.1" />,
                    })}
                </li>
                <li>
                    <strong class={s.guideItemTitle}>
                        {intl.getMessage('setup_devices_router_list_2_title')}
                    </strong>
                    {intl.getMessage('setup_devices_router_list_2')}
                </li>
            </ol>
            <div class={s.guideParagraph}>
                {intl.getMessage('setup_devices_router_desc_2')}
            </div>
        </div>
    </div>
);

const WindowsLayout = () => (
    <div title="Windows">
        <div class={s.title}>{intl.getMessage('setup_devices_windows_title')}</div>
        <div class={s.guideText}>
            <ol class={s.guideList}>
                <li>{intl.getMessage('setup_devices_windows_list_1')}</li>
                <li>{intl.getMessage('setup_devices_windows_list_2')}</li>
                <li>{intl.getMessage('setup_devices_windows_list_3')}</li>
                <li>{intl.getMessage('setup_devices_windows_list_4')}</li>
                <li>{intl.getMessage('setup_devices_windows_list_5')}</li>
                <li>{intl.getMessage('setup_devices_windows_list_6')}</li>
            </ol>
        </div>
    </div>
);

const MacOSLayout = () => (
    <div title="macOS">
        <div class={s.title}>macOS</div>
        <div class={s.guideText}>
            <ol class={s.guideList}>
                <li>{intl.getMessage('setup_devices_macos_list_1')}</li>
                <li>{intl.getMessage('setup_devices_macos_list_2')}</li>
                <li>{intl.getMessage('setup_devices_macos_list_3')}</li>
            </ol>
        </div>
    </div>
);

const AndroidLayout = () => (
    <div title="Android">
        <div class={s.title}>Android</div>
        <div class={s.guideText}>
            <ol class={s.guideList}>
                <li>{intl.getMessage('setup_devices_android_list_1')}</li>
                <li>{intl.getMessage('setup_devices_android_list_2')}</li>
                <li>{intl.getMessage('setup_devices_android_list_3')}</li>
                <li>{intl.getMessage('setup_devices_android_list_4')}</li>
            </ol>
        </div>
    </div>
);

const IOSLayout = () => (
    <div title="iOS">
        <div class={s.title}>iOS</div>
        <div class={s.guideText}>
            <ol class={s.guideList}>
                <li>{intl.getMessage('setup_devices_ios_list_1')}</li>
                <li>{intl.getMessage('setup_devices_ios_list_2')}</li>
                <li>{intl.getMessage('setup_devices_ios_list_3')}</li>
                <li>{intl.getMessage('setup_devices_ios_list_4')}</li>
            </ol>
        </div>
    </div>
);

const getPlatformLayouts = (): PlatformLayouts => ({
    Router: {
        title: intl.getMessage('setup_devices_router_title'),
        icon: 'router',
        component: <RouterLayout />,
    },
    Windows: {
        title: intl.getMessage('setup_devices_windows_title'),
        icon: 'windows',
        component: <WindowsLayout />,
    },
    macOS: {
        title: intl.getMessage('setup_devices_macos_title'),
        icon: 'mac',
        component: <MacOSLayout />,
    },
    Android: {
        title: intl.getMessage('setup_devices_android_title'),
        icon: 'android',
        component: <AndroidLayout />,
    },
    iOS: {
        title: intl.getMessage('setup_devices_ios_title'),
        icon: 'ios',
        component: <IOSLayout />,
    },
});

export const Guide = () => {
    const [activeTabLabel, setActiveTabLabel] = createSignal('Router');

    const platformLayouts = () => getPlatformLayouts();

    const selectOptions = createMemo(() =>
        Object.entries(platformLayouts()).map(([key, value]) => ({
            value: key,
            label: value.title,
            icon: value.icon,
        })),
    );

    const selectedOption = createMemo(() =>
        selectOptions().find((option) => option.value === activeTabLabel()),
    );

    const activeLayout = () => platformLayouts()[activeTabLabel() as keyof typeof platformLayouts];

    return (
        <div class={s.deviceSelectorContainer}>
            <p class={s.selectorDesc}>{intl.getMessage('device_type')}</p>
            <div class={s.deviceSelector}>
                <Select
                    options={selectOptions()}
                    value={selectedOption()}
                    onChange={(option: any) => setActiveTabLabel(option.value)}
                    showIcons={true}
                    size="responsive"
                    height="big"
                />
            </div>
            <Show when={activeLayout()}>
                <div class={s.deviceContent}>{activeLayout().component}</div>
            </Show>
        </div>
    );
};
