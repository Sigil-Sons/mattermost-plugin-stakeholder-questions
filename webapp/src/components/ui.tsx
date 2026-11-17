import React from 'react';

/* eslint-disable @typescript-eslint/no-explicit-any */
export const muted = {color: 'rgba(var(--center-channel-color-rgb), 0.64)'};
export const subtle = 'rgba(var(--center-channel-color-rgb), 0.08)';
export const line = '1px solid rgba(var(--center-channel-color-rgb), 0.16)';

// Markdown through Mattermost's own renderer when available (window.PostUtils); plain text otherwise.
export function Markdown({text}: {text?: string}) {
    if (!text) {
        return null;
    }
    const PU = (window as any).PostUtils;
    try {
        if (PU && PU.formatText && PU.messageHtmlToComponent) {
            return <div className='ssq-md'>{PU.messageHtmlToComponent(PU.formatText(text, {atMentions: true, markdown: true}), {})}</div>;
        }
    } catch (e) {
        // fall through to plain text
    }
    return <div style={{whiteSpace: 'pre-wrap'}}>{text}</div>;
}

export function Thumb({src, alt, big}: {src: string; alt: string; big?: boolean}) {
    return (
        <a href={src} target='_blank' rel='noopener noreferrer' onClick={(e: any) => e.stopPropagation()}>
            <img
                data-testid='ssq-img'
                src={src}
                alt={alt}
                style={{maxWidth: big ? 520 : 160, maxHeight: big ? 320 : 100, borderRadius: 4, border: line, display: 'block', objectFit: 'contain'}}
            />
        </a>
    );
}

export function Modal({title, onClose, children}: {title: string; onClose: () => void; children: any}) {
    return (
        <div
            data-testid='ssq-modal'
            onClick={onClose}
            style={{position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.48)', zIndex: 2000, display: 'flex', alignItems: 'center', justifyContent: 'center'}}
        >
            <div
                role='dialog'
                aria-label={title}
                onClick={(e: any) => e.stopPropagation()}
                style={{background: 'var(--center-channel-bg)', color: 'var(--center-channel-color)', width: 'min(720px, 94vw)',
                    maxHeight: '88vh', overflow: 'auto', borderRadius: 8, boxShadow: '0 20px 32px rgba(0,0,0,0.24)', padding: '20px 24px'}}
            >
                <div style={{display: 'flex', alignItems: 'center', marginBottom: 12}}>
                    <div style={{fontWeight: 600, fontSize: 18, flex: 1}}>{title}</div>
                    <button
                        className='btn btn-tertiary btn-sm'
                        aria-label='Close'
                        data-testid='ssq-close'
                        onClick={onClose}
                    >{'✕'}</button>
                </div>
                {children}
            </div>
        </div>
    );
}
