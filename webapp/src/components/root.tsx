import React, {useEffect, useState} from 'react';

import {View, subscribe} from '../state';

import Results from './results';
import {Modal} from './ui';
import Wizard from './wizard';

// Mounted once via registry.registerRootComponent (React-19-safe); shows the wizard or results modal.
export default function Root() {
    const [view, setView] = useState(null as View);
    useEffect(() => subscribe(setView), []);
    useEffect(() => {
        if (!view) {
            return undefined;
        }
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                setView(null);
            }
        };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [view]);
    if (!view) {
        return null;
    }
    return (
        <Modal
            title={view.kind === 'answer' ? 'Answer questions' : 'Results'}
            onClose={() => setView(null)}
        >
            {view.kind === 'answer' ? <Wizard key={'a' + view.packId} packId={view.packId}/> : <Results key={'r' + view.packId} packId={view.packId}/>}
        </Modal>
    );
}
