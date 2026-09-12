export const statusLabel = status => ({running:'运行中', starting:'启动中', stopped:'已关闭', unmanaged:'需手动关闭', unknown:'状态未知', 'stopped (see status)':'已关闭 · 有提示'}[status] || '状态未知');
export const isActive = row => ['running','starting','unmanaged'].includes(row.status);
export const canOpen = row => row.status === 'stopped' || row.status === 'stopped (see status)';
export const browserLabel = id => ({chrome:'Google Chrome',chromium:'Chromium',custom:'自选浏览器'}[id] || id);
export function visibleRows(rows, filter, query) {
 const q = query.trim().toLocaleLowerCase();
 return rows.filter(row => (filter === 'all' || (filter === 'running' ? isActive(row) : row.type === filter)) &&
  `${row.name} ${browserLabel(row.browser.id)}`.toLocaleLowerCase().includes(q));
}
