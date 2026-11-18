import PackCard from './components/pack_card';
import Root from './components/root';
import {PLUGIN_ID, POST_TYPE} from './manifest';
import {setStore} from './state';

/* eslint-disable @typescript-eslint/no-explicit-any */
// After Mattermost's login, its single-page router "navigates" to redirect_to client-side, which leaves a
// blank app on our standalone answer URL. Detect that and force a real page load (server serves the page).
const ANSWER_PATH = '/plugins/' + PLUGIN_ID + '/answer/';
function hopToAnswerPage() {
    if (window.location.pathname.includes(ANSWER_PATH)) {
        window.location.replace(window.location.href);
    }
}

class Plugin {
    // Only supported registry APIs; Mattermost renders these inside its own React tree (React 19 safe).
    initialize(registry: any, store: any) {
        setStore(store);
        hopToAnswerPage();
        store.subscribe(hopToAnswerPage);
        window.addEventListener('popstate', hopToAnswerPage);
        registry.registerPostTypeComponent(POST_TYPE, PackCard);
        registry.registerRootComponent(Root);
    }
}

(window as any).registerPlugin(PLUGIN_ID, new Plugin());
