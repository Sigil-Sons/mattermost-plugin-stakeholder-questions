/* eslint-disable @typescript-eslint/no-explicit-any */
export type View = {kind: 'answer' | 'results'; packId: string} | null;

const listeners = new Set<(v: View) => void>();
let store: any = null;

export function setStore(s: any) {
    store = s;
}
export function openView(v: View) {
    listeners.forEach((l) => l(v));
}
export function subscribe(l: (v: View) => void): () => void {
    listeners.add(l);
    return () => {
        listeners.delete(l);
    };
}
export function currentUserId(): string {
    return store?.getState()?.entities?.users?.currentUserId || '';
}
export function isSystemAdmin(): boolean {
    const st = store?.getState();
    const me = st?.entities?.users?.profiles?.[currentUserId()];
    return Boolean(me && String(me.roles || '').includes('system_admin'));
}
export function usernameFor(uid: string, fallback?: string): string {
    const p = store?.getState()?.entities?.users?.profiles?.[uid];
    return p?.username || fallback || uid.slice(0, 6);
}
