import {HOST, requestFor} from './request.js';
const busy = new Set();
chrome.action.onClicked.addListener(async tab => {
 if (busy.has(tab.id)) return;
 busy.add(tab.id);
 try {
  const request = requestFor(tab);
  await chrome.action.setBadgeBackgroundColor({tabId: tab.id, color: '#205941'});
  await chrome.action.setBadgeText({tabId: tab.id, text: '…'});
  const response = await chrome.runtime.sendNativeMessage(HOST, request);
  if (!response?.ok) throw new Error(response?.error || '本机组件未返回成功结果，请检查会话状态。');
  await chrome.action.setBadgeText({tabId: tab.id, text: ''});
  await chrome.action.setTitle({tabId: tab.id, title: 'New Session · 已打开独立临时窗口'});
 } catch (error) {
  // Do not persist URLs or log errors that could contain page information.
  await chrome.action.setBadgeText({tabId: tab.id, text: '!'}).catch(()=>{});
  await chrome.action.setTitle({tabId: tab.id, title: 'New Session 未完成，请查看设置页的安装及排查步骤'}).catch(()=>{});
  await chrome.runtime.openOptionsPage();
 } finally {busy.delete(tab.id);}
});
chrome.runtime.onInstalled.addListener(({reason}) => {
 if (reason === 'install') chrome.runtime.openOptionsPage();
});
