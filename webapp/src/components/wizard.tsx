import React, {useEffect, useState} from 'react';

import {Answer, Pack, Question, fileUrl, getPack, saveAnswer, uploadImage} from '../api';
import {currentUserId, openView, usernameFor} from '../state';

import {Markdown, Thumb, line, muted, subtle} from './ui';

/* eslint-disable @typescript-eslint/no-explicit-any */
type Draft = {option_ids: string[]; other: string; otherOn: boolean; images: string[]};

function draftFrom(a?: Answer): Draft {
    return {option_ids: a?.option_ids || [], other: a?.other_text || '', otherOn: Boolean(a?.other_text), images: a?.image_file_ids || []};
}

function labelsFor(q: Question, a: Answer): string {
    if (a.skipped) {
        return 'Skipped';
    }
    const ls = a.option_ids.map((id) => q.options.find((o) => o.id === id)?.label || id);
    if (a.other_text) {
        ls.push('Other: ' + a.other_text);
    }
    return ls.join(', ');
}

// Answer wizard (SPEC 4.2): one question at a time, saved on Next, revisable until the pack closes.
export default function Wizard({packId}: {packId: string}) {
    const me = currentUserId();
    const [pack, setPack] = useState(null as Pack | null);
    const [idx, setIdx] = useState(0);
    const [drafts, setDrafts] = useState({} as Record<string, Draft>);
    const [others, setOthers] = useState(null as Pack | null);
    const [showOthers, setShowOthers] = useState(false);
    const [busy, setBusy] = useState(false);
    const [err, setErr] = useState('');
    const [done, setDone] = useState(false);

    useEffect(() => {
        getPack(packId).then((p: Pack) => {
            setPack(p);
            const d: Record<string, Draft> = {};
            p.questions.forEach((q) => {
                d[q.id] = draftFrom(p.answers?.[me]?.[q.id]);
            });
            setDrafts(d);
            const firstOpen = p.questions.findIndex((q) => !p.answers?.[me]?.[q.id]);
            setIdx(firstOpen >= 0 ? firstOpen : 0);
        }).catch((e: Error) => setErr(e.message));
    }, [packId]);

    if (err && !pack) {
        return <div style={{color: 'var(--error-text)'}}>{err}</div>;
    }
    if (!pack) {
        return <div style={muted}>{'Loading…'}</div>;
    }
    const isRespondent = pack.respondent_user_ids.includes(me);
    const editable = isRespondent && pack.status === 'open';
    const q = pack.questions[idx];
    const d = drafts[q.id] || draftFrom();
    const n = pack.questions.length;
    const answeredCount = Object.keys(pack.answers?.[me] || {}).length;

    const setDraft = (nd: Partial<Draft>) => setDrafts({...drafts, [q.id]: {...d, ...nd}});
    const toggle = (oid: string) => {
        if (q.selection === 'single') {
            setDraft({option_ids: [oid], otherOn: false});
        } else {
            setDraft({option_ids: d.option_ids.includes(oid) ? d.option_ids.filter((x) => x !== oid) : [...d.option_ids, oid]});
        }
    };
    const toggleOther = () => {
        if (q.selection === 'single') {
            setDraft({otherOn: true, option_ids: []});
        } else {
            setDraft({otherOn: !d.otherOn});
        }
    };

    const revealOthers = () => {
        if (showOthers) {
            setShowOthers(false);
            return;
        }
        getPack(pack.id, true).then((p: Pack) => {
            setOthers(p);
            setShowOthers(true);
        }).catch((e: Error) => setErr(e.message));
    };

    const save = async (skip = false): Promise<boolean> => {
        if (!editable) {
            return true;
        }
        setBusy(true);
        setErr('');
        try {
            const p = await saveAnswer(pack.id, {question_id: q.id, option_ids: d.otherOn && q.selection === 'single' ? [] : d.option_ids,
                other_text: d.otherOn && d.other.trim() ? d.other.trim() : null, image_file_ids: d.images, skip});
            setPack(p);
            return true;
        } catch (e: any) {
            setErr(e.message);
            return false;
        } finally {
            setBusy(false);
        }
    };
    const next = async (skip = false) => {
        if (!(await save(skip))) {
            return;
        }
        setShowOthers(false);
        if (idx < n - 1) {
            setIdx(idx + 1);
        } else {
            setDone(true);
        }
    };
    const onFile = async (e: any) => {
        const f: File | undefined = e.target.files?.[0];
        e.target.value = '';
        if (!f) {
            return;
        }
        setBusy(true);
        try {
            const id = await uploadImage(pack.channel_id, f);
            setDraft({images: [...d.images, id]});
        } catch (x: any) {
            setErr(x.message);
        } finally {
            setBusy(false);
        }
    };

    if (done) {
        const complete = pack.completed_user_ids.includes(me);
        return (
            <div data-testid='ssq-done'>
                <div style={{fontSize: 16, fontWeight: 600, marginBottom: 6}}>{complete ? 'All answered ✅' : 'Saved'}</div>
                <div style={muted}>
                    {pack.status === 'open' ? 'You can revise your answers until the pack closes.' : 'This pack is closed.'}
                </div>
                <div style={{marginTop: 16, display: 'flex', gap: 8}}>
                    <button
                        className='btn btn-primary'
                        data-testid='ssq-view-results'
                        onClick={() => openView({kind: 'results', packId: pack.id})}
                    >{'View results'}</button>
                    <button
                        className='btn btn-tertiary'
                        onClick={() => {
                            setDone(false);
                            setIdx(0);
                        }}
                    >{'Review my answers'}</button>
                </div>
            </div>
        );
    }

    const othersForQ = showOthers && others ? Object.entries(others.answers || {}).
        filter(([uid, byQ]) => uid !== me && byQ[q.id]).map(([uid, byQ]) => ({uid, a: byQ[q.id]})) : [];

    return (
        <div data-testid='ssq-wizard'>
            {!editable && (
                <div style={{background: subtle, padding: '6px 10px', borderRadius: 4, marginBottom: 10, ...muted}}>
                    {isRespondent ? 'This pack is ' + pack.status + '; answers are read-only.' : 'You are not a respondent on this pack.'}
                </div>
            )}
            <div style={{...muted, fontSize: 12, textTransform: 'uppercase', letterSpacing: 0.4}}>
                {'Q '}{idx + 1}{' of '}{n}{' · '}{answeredCount}{'/'}{n}{' answered'}{q.selection === 'multi' ? ' · pick any' : ''}
            </div>
            <div
                data-testid='ssq-prompt'
                style={{fontSize: 16, fontWeight: 600, margin: '4px 0 8px'}}
            >{q.prompt}</div>
            <Markdown text={q.help_markdown}/>
            {q.help_file_ids?.length ? (
                <div style={{display: 'flex', gap: 8, flexWrap: 'wrap', margin: '8px 0'}}>
                    {q.help_file_ids.map((f) => (
                        <Thumb
                            key={f}
                            big={true}
                            src={fileUrl(pack.id, f)}
                            alt='context'
                        />
                    ))}
                </div>
            ) : null}
            <div style={{display: 'flex', flexDirection: 'column', gap: 6, margin: '12px 0'}}>
                {q.options.map((o) => {
                    const on = d.option_ids.includes(o.id) && !(q.selection === 'single' && d.otherOn);
                    const img = o.image_file_id ? fileUrl(pack.id, o.image_file_id) : (o.image_url || '');
                    return (
                        <label
                            key={o.id}
                            data-testid={'ssq-option-' + q.id + '-' + o.id}
                            style={{display: 'flex', gap: 10, alignItems: 'flex-start', padding: '8px 10px', borderRadius: 4, cursor: editable ? 'pointer' : 'default',
                                border: on ? '1px solid var(--button-bg)' : line, background: on ? 'rgba(var(--button-bg-rgb), 0.08)' : 'transparent'}}
                        >
                            <input
                                type={q.selection === 'single' ? 'radio' : 'checkbox'}
                                name={'ssq-' + q.id}
                                checked={on}
                                disabled={!editable}
                                onChange={() => toggle(o.id)}
                                style={{marginTop: 4}}
                            />
                            <div style={{flex: 1}}>
                                <div style={{fontWeight: 600}}>
                                    {o.label}
                                    {o.recommended ? <span style={{marginLeft: 8, fontSize: 11, padding: '1px 6px', borderRadius: 10, background: 'rgba(var(--button-bg-rgb), 0.16)', color: 'var(--button-bg)'}}>{'Recommended'}</span> : null}
                                </div>
                                {o.description ? <div style={muted}>{o.description}</div> : null}
                                {img ? <div style={{marginTop: 6}}><Thumb src={img} alt={o.label}/></div> : null}
                            </div>
                        </label>
                    );
                })}
                {q.allow_other && (
                    <div style={{padding: '8px 10px', borderRadius: 4, border: d.otherOn ? '1px solid var(--button-bg)' : line}}>
                        <label style={{display: 'flex', gap: 10, alignItems: 'center', cursor: editable ? 'pointer' : 'default', margin: 0}}>
                            <input
                                type={q.selection === 'single' ? 'radio' : 'checkbox'}
                                name={'ssq-' + q.id}
                                data-testid={'ssq-other-' + q.id}
                                checked={d.otherOn}
                                disabled={!editable}
                                onChange={toggleOther}
                            />
                            <span style={{fontWeight: 600}}>{'Other'}</span>
                        </label>
                        {d.otherOn && (
                            <textarea
                                className='form-control'
                                data-testid={'ssq-other-text-' + q.id}
                                value={d.other}
                                disabled={!editable}
                                maxLength={2000}
                                placeholder='Your answer'
                                onChange={(e: any) => setDraft({other: e.target.value})}
                                style={{marginTop: 6, minHeight: 56}}
                            />
                        )}
                    </div>
                )}
            </div>
            <div style={{display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', marginBottom: 8}}>
                {d.images.map((f) => (
                    <div
                        key={f}
                        style={{position: 'relative'}}
                    >
                        <Thumb
                            src={fileUrl(pack.id, f)}
                            alt='your image'
                        />
                        {editable && (
                            <button
                                className='btn btn-tertiary btn-xs'
                                style={{position: 'absolute', top: 2, right: 2}}
                                onClick={() => setDraft({images: d.images.filter((x) => x !== f)})}
                            >{'✕'}</button>
                        )}
                    </div>
                ))}
                {editable && d.images.length < 5 && (
                    <label
                        className='btn btn-tertiary btn-sm'
                        style={{margin: 0}}
                    >
                        {'🖼 Attach image'}
                        <input
                            type='file'
                            accept='image/*'
                            data-testid='ssq-file-input'
                            onChange={onFile}
                            style={{display: 'none'}}
                        />
                    </label>
                )}
            </div>
            <div style={{borderTop: line, paddingTop: 10, marginTop: 4}}>
                <button
                    className='btn btn-link btn-sm'
                    data-testid='ssq-show-others'
                    style={{padding: 0}}
                    onClick={revealOthers}
                >{showOthers ? 'Hide others’ answers' : 'Show others’ answers'}</button>
                {showOthers && (
                    <div
                        data-testid='ssq-others'
                        style={{marginTop: 6}}
                    >
                        {othersForQ.length === 0 ? <div style={muted}>{'No one else has answered this yet.'}</div> : othersForQ.map(({uid, a}) => (
                            <div
                                key={uid}
                                style={{display: 'flex', gap: 8, alignItems: 'flex-start', padding: '4px 0'}}
                            >
                                <strong>{'@'}{usernameFor(uid, others?.respondent_usernames[others.respondent_user_ids.indexOf(uid)])}</strong>
                                <span>{labelsFor(q, a)}</span>
                                {a.image_file_ids.map((f) => (
                                    <Thumb
                                        key={f}
                                        src={fileUrl(pack.id, f, true)}
                                        alt='answer image'
                                    />
                                ))}
                            </div>
                        ))}
                    </div>
                )}
            </div>
            {err && <div style={{color: 'var(--error-text)', marginTop: 8}}>{err}</div>}
            <div style={{display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 16}}>
                {pack.allow_skip && editable && (
                    <button
                        className='btn btn-tertiary'
                        disabled={busy}
                        onClick={() => next(true)}
                    >{'Skip'}</button>
                )}
                <button
                    className='btn btn-tertiary'
                    data-testid='ssq-back'
                    disabled={idx === 0 || busy}
                    onClick={() => {
                        setShowOthers(false);
                        setIdx(idx - 1);
                    }}
                >{'Back'}</button>
                <button
                    className='btn btn-primary'
                    data-testid={idx === n - 1 ? 'ssq-submit' : 'ssq-next'}
                    disabled={busy}
                    onClick={() => next(false)}
                >{idx === n - 1 ? (editable ? 'Submit' : 'Done') : 'Next'}</button>
            </div>
        </div>
    );
}
