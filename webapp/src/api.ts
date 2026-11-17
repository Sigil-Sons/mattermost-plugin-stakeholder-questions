import {PLUGIN_ID} from './manifest';

/* eslint-disable @typescript-eslint/no-explicit-any */
export type Option = {id: string; label: string; description?: string; recommended?: boolean; image_file_id?: string | null; image_url?: string | null};
export type Question = {id: string; prompt: string; help_markdown?: string; help_file_ids: string[]; selection: 'single' | 'multi'; allow_other: boolean; options: Option[]};
export type Answer = {option_ids: string[]; other_text?: string | null; image_file_ids: string[]; updated_at: number; skipped?: boolean};
export type Pack = {
    id: string; channel_id: string; post_id: string; title: string; summary?: string; author_user_id: string;
    respondent_user_ids: string[]; respondent_usernames: string[]; min_respondents: number; status: string;
    allow_skip: boolean; allow_revise_until_lock: boolean; questions: Question[];
    answers: Record<string, Record<string, Answer>>; completed_user_ids: string[]; created_at: number;
};

const root = () => ((window as any).basename || '');
const base = () => root() + '/plugins/' + PLUGIN_ID + '/api/v1';

function csrf(): string {
    const m = document.cookie.match(/(?:^|; )MMCSRF=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : '';
}

function headers(json: boolean): Record<string, string> {
    const h: Record<string, string> = {'X-Requested-With': 'XMLHttpRequest', 'X-CSRF-Token': csrf()};
    if (json) {
        h['Content-Type'] = 'application/json';
    }
    return h;
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
    const r = await fetch(base() + path, {method, credentials: 'include', headers: headers(body !== undefined),
        body: body === undefined ? undefined : JSON.stringify(body)});
    const t = await r.text();
    let d: any = {};
    try {
        d = t ? JSON.parse(t) : {};
    } catch (e) {
        d = {error: t};
    }
    if (!r.ok) {
        throw new Error(d.error || ('HTTP ' + r.status));
    }
    return d as T;
}

export const getPack = (id: string, showOthers = false) => call<Pack>('GET', '/packs/' + id + (showOthers ? '?show_others=1' : ''));
export const saveAnswer = (id: string, body: {question_id: string; option_ids: string[]; other_text: string | null; image_file_ids: string[]; skip?: boolean}) =>
    call<Pack>('POST', '/packs/' + id + '/answers', body);
export const lockPack = (id: string) => call<Pack>('POST', '/packs/' + id + '/lock');

export async function getSummary(id: string): Promise<string> {
    const r = await fetch(base() + '/packs/' + id + '/summary.md', {credentials: 'include', headers: headers(false)});
    if (!r.ok) {
        throw new Error('HTTP ' + r.status);
    }
    return r.text();
}

export const summaryUrl = (id: string) => base() + '/packs/' + id + '/summary.md';
export const fileUrl = (packId: string, fileId: string, showOthers = false) =>
    base() + '/packs/' + packId + '/files/' + fileId + (showOthers ? '?show_others=1' : '');

export async function uploadImage(channelId: string, file: File): Promise<string> {
    const fd = new FormData();
    fd.append('files', file, file.name);
    fd.append('channel_id', channelId);
    const r = await fetch(root() + '/api/v4/files?channel_id=' + encodeURIComponent(channelId),
        {method: 'POST', credentials: 'include', headers: headers(false), body: fd});
    const d: any = await r.json().catch(() => ({}));
    if (!r.ok || !d.file_infos || !d.file_infos.length) {
        throw new Error(d.message || 'upload failed');
    }
    return d.file_infos[0].id;
}
