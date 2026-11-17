import PackCard from './components/pack_card';
import Root from './components/root';
import {PLUGIN_ID, POST_TYPE} from './manifest';
import {setStore} from './state';

/* eslint-disable @typescript-eslint/no-explicit-any */
class Plugin {
    // Only supported registry APIs; Mattermost renders these inside its own React tree (React 19 safe).
    initialize(registry: any, store: any) {
        setStore(store);
        registry.registerPostTypeComponent(POST_TYPE, PackCard);
        registry.registerRootComponent(Root);
    }
}

(window as any).registerPlugin(PLUGIN_ID, new Plugin());
