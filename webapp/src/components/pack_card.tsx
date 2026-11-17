import React, {useEffect, useState} from 'react';

import {Pack, getPack} from '../api';
import {currentUserId, isSystemAdmin, openView} from '../state';

/* eslint-disable @typescript-eslint/no-explicit-any */
type Props = {post: {id: string; props: Record<string, any>; create_at: number}};

const STATUS_LABEL: Record<string, string> = {open: 'Open', complete: 'Complete', locked: 'Locked', expired: 'Expired'};
const STATUS_BAR: Record<string, string> = {
    open: 'var(--button-bg)',
    complete: 'var(--online-indicator)',
    locked: 'rgba(var(--center-channel-color-rgb), 0.48)',
    expired: 'var(--dnd-indicator)',
};

// Compact channel card (SPEC 4.1). Renders from post props (live via post edits); never dumps questions inline.
export default function PackCard({post}: Props) {
    const p = post.props || {};
    const me = currentUserId();
    const respondents: string[] = p.ssq_respondents || [];
    const respondentIds: string[] = p.ssq_respondent_ids || [];
    const completed: string[] = p.ssq_completed || [];
    const completedIds: string[] = p.ssq_completed_ids || [];
    const status: string = p.ssq_status || 'open';
    const total: number = p.ssq_question_count || 0;
    const isRespondent = respondentIds.includes(me);
    const iDone = completedIds.includes(me);
    const isAuthor = p.ssq_author_id === me;
    const [mine, setMine] = useState(0);

    useEffect(() => {
        if (!isRespondent || !p.ssq_pack_id) {
            return;
        }
        getPack(p.ssq_pack_id).then((pk: Pack) => setMine(Object.keys(pk.answers?.[me] || {}).length)).catch(() => undefined);
    }, [p.ssq_pack_id, p.ssq_updated_at, isRespondent]);

    const label = status === 'open' && completed.length > 0 ? 'In progress' : (STATUS_LABEL[status] || status);
    const created = new Date(p.ssq_created_at || post.create_at).toISOString().slice(0, 10);
    const muted = {color: 'rgba(var(--center-channel-color-rgb), 0.72)'};
    const canResults = isAuthor || isSystemAdmin() || (isRespondent && iDone) || status !== 'open';

    let answerLabel = 'Answer questions';
    if (isRespondent && status === 'open') {
        answerLabel = iDone ? 'Revise answers' : 'Answer (' + mine + '/' + total + ' done)';
    } else if (isRespondent) {
        answerLabel = 'View my answers';
    }

    return (
        <div
            data-testid='ssq-card'
            data-pack-id={p.ssq_pack_id}
            data-status={status}
            style={{
                // shorthand first, then the left edge; the other order wipes the status color
                border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
                borderLeft: `4px solid ${STATUS_BAR[status] || STATUS_BAR.open}`,
                borderRadius: 4,
                padding: '10px 14px',
                margin: '4px 0',
                maxWidth: 560,
                lineHeight: '20px',
            }}
        >
            <div style={{fontWeight: 600}}>{'📋 Stakeholder pack · '}{p.ssq_title}</div>
            {p.ssq_summary ? <div style={muted}>{p.ssq_summary}</div> : null}
            <div style={muted}>
                {'Need: '}{respondents.map((r) => '@' + r).join(' ')}
                {' · '}{total}{total === 1 ? ' question' : ' questions'}
                {' · Quorum '}{completed.length}{'/'}{p.ssq_min_respondents}
            </div>
            <div data-testid='ssq-progress'>
                {'Progress: '}
                {respondents.map((r, i) => (
                    <span key={r}>{i ? ' · ' : ''}{r}{' '}{completed.includes(r) ? '✅' : '⏳'}</span>
                ))}
            </div>
            <div style={{marginTop: 8, display: 'flex', gap: 8}}>
                {isRespondent && (
                    <button
                        className='btn btn-primary btn-sm'
                        data-testid='ssq-answer-btn'
                        onClick={() => openView({kind: 'answer', packId: p.ssq_pack_id})}
                    >{answerLabel}</button>
                )}
                <button
                    className='btn btn-tertiary btn-sm'
                    data-testid='ssq-results-btn'
                    disabled={!canResults}
                    title={canResults ? '' : 'Results open after you finish answering'}
                    onClick={() => openView({kind: 'results', packId: p.ssq_pack_id})}
                >{'View results'}</button>
            </div>
            <div style={{...muted, fontSize: 12, marginTop: 6}}>{'Status: '}<span data-testid='ssq-status'>{label}</span>{' · '}{created}</div>
        </div>
    );
}
