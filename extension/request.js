export const HOST = 'io.github.aromaw.browser_session';
export function requestFor(tab) {
 const raw = tab?.url;
 if (typeof raw !== 'string' || raw.length > 8192) throw new Error('请在普通 HTTP/HTTPS 网页上使用。');
 const url = new URL(raw);
 if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password) throw new Error('请在普通 HTTP/HTTPS 网页上使用。');
 return {op: 'new-session', url: raw};
}
