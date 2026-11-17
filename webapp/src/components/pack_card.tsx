import React from 'react';

type Props = {post: {id: string; props: Record<string, any>; create_at: number}};

const STATUS_LABEL: Record<string, string> = {open: 'Open', complete: 'Complete', locked: 'Locked', expired: 'Expired'};
const STATUS_BAR: Record<string, string> = {
    open: 'var(--button-bg)',
    complete: 'var(--online-indicator)',
    locked: 'rgba(var(--center-channel-color-rgb), 0.48)',
    expired: 'var(--dnd-indicator)',
};

// Compact channel card (SPEC 4.1). Renders from post props only; never dumps questions inline.
export default function PackCard({post}: Props) {
    const p = post.props || {};
    const respondents: string[] = p.ssq_respondents || [];
    const completed: string[] = p.ssq_completed || [];
    const status: string = p.ssq_status || 'open';
    const status_ = completed.length > 0 && status === 'open' ? 'In progress' : (STATUS_LABEL[status] || status);
    const created = new Date(p.ssq_created_at || post.create_at).toISOString().slice(0, 10);
    const muted = {color: 'rgba(var(--center-channel-color-rgb), 0.72)'};

    return (
        <div
            data-testid='ssq-card'
            data-pack-id={p.ssq_pack_id}
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
                {' · '}{p.ssq_question_count}{p.ssq_question_count === 1 ? ' question' : ' questions'}
                {' · Quorum '}{completed.length}{'/'}{p.ssq_min_respondents}
            </div>
            <div data-testid='ssq-progress'>
                {'Progress: '}
                {respondents.map((r, i) => (
                    <span key={r}>{i ? ' · ' : ''}{r}{' '}{completed.includes(r) ? '✅' : '⏳'}</span>
                ))}
            </div>
            <div style={{marginTop: 8, display: 'flex', gap: 8}}>
                <button
                    className='btn btn-primary btn-sm'
                    disabled={true}
                    title='Answer wizard arrives in slice 3'
                >{'Answer questions'}</button>
                <button
                    className='btn btn-tertiary btn-sm'
                    disabled={true}
                >{'View results'}</button>
            </div>
            <div style={{...muted, fontSize: 12, marginTop: 6}}>{'Status: '}{status_}{' · '}{created}</div>
        </div>
    );
}
