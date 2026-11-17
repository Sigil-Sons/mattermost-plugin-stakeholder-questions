import PackCard from './components/pack_card';
import {PLUGIN_ID, POST_TYPE} from './manifest';

class Plugin {
    // Only supported registry APIs; Mattermost renders these inside its own React tree.
    initialize(registry: any) {
        registry.registerPostTypeComponent(POST_TYPE, PackCard);
    }
}

(window as any).registerPlugin(PLUGIN_ID, new Plugin());
