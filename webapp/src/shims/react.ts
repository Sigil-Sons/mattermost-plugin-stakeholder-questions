// Mattermost exposes its own React (19 on MM 12) on window. Never bundle a second copy,
// and never call ReactDOM.render (removed in React 19; see jitsi-2.1.0-sigil.1).
/* eslint-disable @typescript-eslint/no-explicit-any */
const R: any = (window as any).React;
export default R;
export const useState: any = R.useState;
export const useEffect: any = R.useEffect;
export const useCallback: any = R.useCallback;
export const useMemo: any = R.useMemo;
