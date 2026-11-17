import React, {useEffect, useState} from 'react';

import {Pack, fileUrl, getPack, getSummary, lockPack, summaryUrl} from '../api';
import {currentUserId, isSystemAdmin, usernameFor} from '../state';

import {Thumb, line, muted, subtle} from './ui';

/* eslint-disable @typescript-eslint/no-explicit-any */
// Results view (SPEC 4.3): histogram + named breakdown + markdown export. Fetching it is an explicit opt-in.
export default function Results({packId}: {packId: string}) {
    const me = currentUserId();
    const [pack, setPack] = useState(null as Pack | null);
    const [err, setErr] = useState('');
    const [copied, setCopied] = useState('');

    useEffect(() => {
        getPack(packId, true).then(setPack).catch((e: Error) => setErr(e.message));
    }, [packId]);

    if (err && !pack) {
        return <div style={{color: 'var(--error-text)'}}>{err}</div>;
    }
    if (!pack) {
        return <div style={muted}>{'Loading…'}</div>;
    }
    const name = (uid: string) => '@' + usernameFor(uid, pack.respondent_usernames[pack.respondent_user_ids.indexOf(uid)]);
    const respondents = pack.respondent_user_ids;
    const canLock = pack.status === 'open' && (pack.author_user_id === me || isSystemAdmin());

    const copy = async () => {
        try {
            const md = await getSummary(pack.id);
            await navigator.clipboard.writeText(md);
            setCopied('Copied summary.md');
        } catch (e: any) {
            setCopied('Copy failed: open summary.md instead');
        }
    };

    return (
        <div data-testid='ssq-results'>
            <div style={{...muted, marginBottom: 12}}>
                {'Status: '}<strong data-testid='ssq-results-status'>{pack.status}</strong>{' · '}
                {pack.completed_user_ids.length}{'/'}{respondents.length}{' complete · quorum '}{pack.min_respondents}
            </div>
            {pack.questions.map((q, i) => {
                const answers = respondents.map((u) => ({u, a: pack.answers?.[u]?.[q.id]})).filter((x) => x.a && !x.a.skipped);
                const total = answers.length || 1;
                return (
                    <div
                        key={q.id}
                        data-testid={'ssq-result-' + q.id}
                        style={{marginBottom: 18}}
                    >
                        <div style={{fontWeight: 600, marginBottom: 6}}>{i + 1}{'. '}{q.prompt}</div>
                        {q.options.map((o) => {
                            const who = answers.filter((x) => x.a!.option_ids.includes(o.id)).map((x) => x.u);
                            const pct = Math.round((who.length / total) * 100);
                            return (
                                <div
                                    key={o.id}
                                    style={{marginBottom: 6}}
                                >
                                    <div style={{display: 'flex', justifyContent: 'space-between', fontSize: 13}}>
                                        <span>{o.label}</span>
                                        <span style={muted}>{who.length}{who.length ? ' · ' + who.map(name).join(', ') : ''}</span>
                                    </div>
                                    <div style={{height: 8, background: subtle, borderRadius: 4, overflow: 'hidden'}}>
                                        <div style={{width: pct + '%', height: '100%', background: 'var(--button-bg)'}}/>
                                    </div>
                                </div>
                            );
                        })}
                        {answers.filter((x) => x.a!.other_text).map((x) => (
                            <div
                                key={x.u}
                                style={{fontSize: 13}}
                            >{'Other · '}<strong>{name(x.u)}</strong>{': '}{x.a!.other_text}</div>
                        ))}
                        {answers.some((x) => x.a!.image_file_ids?.length) && (
                            <div style={{display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 6}}>
                                {answers.flatMap((x) => x.a!.image_file_ids.map((f) => (
                                    <figure
                                        key={f}
                                        style={{margin: 0}}
                                    >
                                        <Thumb
                                            src={fileUrl(pack.id, f, true)}
                                            alt={name(x.u)}
                                        />
                                        <figcaption style={{...muted, fontSize: 12, marginTop: 2}}>{name(x.u)}</figcaption>
                                    </figure>
                                )))}
                            </div>
                        )}
                        {answers.length === 0 && <div style={muted}>{'No answers yet.'}</div>}
                    </div>
                );
            })}
            <div style={{borderTop: line, paddingTop: 12, display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap'}}>
                <button
                    className='btn btn-tertiary btn-sm'
                    data-testid='ssq-copy-md'
                    onClick={copy}
                >{'Copy markdown summary'}</button>
                <a
                    className='btn btn-tertiary btn-sm'
                    href={summaryUrl(pack.id)}
                    target='_blank'
                    rel='noopener noreferrer'
                >{'Open summary.md'}</a>
                {canLock && (
                    <button
                        className='btn btn-danger btn-sm'
                        data-testid='ssq-lock'
                        onClick={() => lockPack(pack.id).then(setPack).catch((e: Error) => setErr(e.message))}
                    >{'Lock pack'}</button>
                )}
                {copied && <span style={muted}>{copied}</span>}
            </div>
            {err && <div style={{color: 'var(--error-text)', marginTop: 8}}>{err}</div>}
        </div>
    );
}
