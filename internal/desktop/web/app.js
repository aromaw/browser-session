import { statusLabel, isActive, canOpen, browserLabel, visibleRows } from './model.js';

const $ = id => document.getElementById(id);
const paths = {
 grid:'M3 3h6v6H3z M15 3h6v6h-6z M3 15h6v6H3z M15 15h6v6h-6z',
 layers:'m12 3 10 5-10 5L2 8z M2 12l10 5 10-5 M2 16l10 5 10-5',
 spark:'m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5z',
 activity:'M2 12h5l3-8 4 16 3-8h5', shield:'M12 3 4 6v6c0 5 8 9 8 9s8-4 8-9V6z m-4 9 3 3 5-6',
 info:'M12 11v6 M12 7h.01 M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0',
 plus:'M12 5v14 M5 12h14', search:'M21 21l-5-5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0',
 refresh:'M20 7v5h-5 M4 17v-5h5 M6 6a8 8 0 0 1 14 6 M18 18a8 8 0 0 1-14-6',
 broom:'m16 3-6 10 M8 11l8 5-4 6H2l4-9z M9 16l-3 6', lock:'M6 10h12v11H6z M8 10V6a4 4 0 0 1 8 0v4',
 x:'M6 6l12 12 M6 18 18 6', more:'M5 12h.01 M12 12h.01 M19 12h.01',
 trash:'M3 6h18 M9 6V3h6v3 M6 6l1 15h10l1-15 M10 10v7 M14 10v7'
};
function icon(name) {
 const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');
 svg.setAttribute('viewBox','0 0 24 24'); svg.setAttribute('class','icon'); svg.setAttribute('aria-hidden','true');
 const p=document.createElementNS(svg.namespaceURI,'path'); p.setAttribute('d',paths[name] || paths.layers); svg.append(p); return svg;
}
for(const el of document.querySelectorAll('[data-icon]')) el.append(icon(el.dataset.icon));
const node=(tag,cls,text)=>{const el=document.createElement(tag); if(cls)el.className=cls; if(text!==undefined)el.textContent=text; return el;};
const api=()=>{const app=window.go?.desktop?.App; if(!app) throw new Error('无法连接桌面程序。请从 Browser Sessions 应用打开此界面。'); return app;};
let rows=[],filter='all',query='',ready=false,refreshing=false,creating=false,deleting=false,kind='persistent',detailID='',deleteID='',lastRender='';
const busy=new Map();
let toastTimer;
function toast(message) { $('toast').textContent=message; $('toast').hidden=false; clearTimeout(toastTimer); toastTimer=setTimeout(()=>$('toast').hidden=true,4500); }
function showError(err) { $('error-text').textContent=err?.message || String(err); $('error-banner').hidden=false; }
function formError(id,err) { $(id).textContent=err?.message || String(err); $(id).hidden=false; }
function render() {
 const shown=visibleRows(rows,filter,query);
 const signature=JSON.stringify([rows,filter,query,[...busy],ready]);
 if(signature===lastRender) return; lastRender=signature;
 // Keep a keyboard user's focused action through polling and state updates.
 const focused=document.activeElement?.dataset?.focusKey;
 for(const k of ['all','persistent','temporary','running']) {
  const count=k==='all'?rows.length:k==='running'?rows.filter(isActive).length:rows.filter(r=>r.type===k).length;
  $('count-'+k).textContent=count;
 }
 $('total').textContent=ready?rows.length:'—'; $('active').textContent=ready?rows.filter(isActive).length:'—';
 $('session-area').setAttribute('aria-busy',String(!ready));
 $('rows').replaceChildren();
 for(const row of shown) {
  const activity=busy.get(row.id),item=node('article','session-row'+(activity?' busy':'')); item.dataset.id=row.id;
  const identity=node('button','session-identity'); identity.type='button'; identity.title='查看 '+row.name+' 的详细信息'; identity.dataset.focusKey=row.id+'-detail';
  const tone=Array.from(row.name).reduce((n,c)=>n+c.charCodeAt(0),0)%4;
  const avatar=node('span','avatar '+(row.type==='temporary'?'temporary':'tone-'+tone),row.type==='temporary'?'':row.name.slice(0,2).toUpperCase());
  if(row.type==='temporary')avatar.append(icon('spark'));
  const identityText=node('span','identity-text'),name=node('span','session-name',row.name),meta=node('span','session-meta');
  meta.append(node('span','',browserLabel(row.browser.id)),node('i','divider'),node('span','',row.type==='temporary'?'临时':'持久'));
  identityText.append(name,meta);identity.append(avatar,identityText);identity.addEventListener('click',()=>showDetail(row.id));
  const status=node('span','status '+row.status.split(' ')[0]);status.append(node('i','dot'),node('span','',activity || statusLabel(row.status)));
  const actions=node('div','row-actions');
  const close=row.status==='running';
  const action=node('button',close?'row-close':'row-open',close?'关闭':'打开');action.type='button';action.dataset.focusKey=row.id+'-action';
  action.disabled=!!activity || (!close && !canOpen(row));action.setAttribute('aria-label',(close?'关闭 ':'打开 ')+row.name);
  action.addEventListener('click',()=>operate(row,close));
  const more=node('button','icon-button');more.type='button';more.append(icon('more'));more.setAttribute('aria-label',row.name+' 的会话详情');more.dataset.focusKey=row.id+'-more';more.addEventListener('click',()=>showDetail(row.id));
  actions.append(action,more);item.append(identity,status,actions);$('rows').append(item);
 }
 if(focused)for(const button of $('rows').querySelectorAll('button')) if(button.dataset.focusKey===focused && !button.disabled)button.focus({preventScroll:true});
 $('empty').hidden=shown.length>0;
 $('empty-title').textContent=!ready?'正在读取会话…':rows.length===0?'从第一个独立空间开始':'没有找到匹配的会话';
 $('empty-copy').textContent=!ready?'连接本机的浏览器空间。':rows.length===0?'创建 personal 或 work，在不同窗口登录不同账号。':'试试其他名称，或切换左侧的分类。';
 $('empty-create').hidden=!ready || rows.length>0;
 $('shown-count').textContent=ready?`${shown.length} 个会话`:'';
}
async function refresh(manual=false) {
 if(refreshing)return; refreshing=true; $('refresh-button').disabled=true;
 try {
  const state=await api().Snapshot();rows=state.rows || [];ready=true;
  $('version').textContent='Browser Sessions · '+state.version;$('data-root').textContent=state.root;
  render(); if(manual)toast('会话状态已更新');
 } catch(err) { showError(err); if(!ready){$('empty-title').textContent='暂时无法读取会话';$('empty-copy').textContent='请查看上方提示，处理后点击刷新。';$('session-area').setAttribute('aria-busy','false');} }
 finally { refreshing=false;$('refresh-button').disabled=false; }
}
async function operate(row,close) {
 if(busy.has(row.id))return;
 busy.set(row.id,close?'正在关闭…':'正在打开…');render();
 try { if(close)await api().CloseSession(row.id);else await api().OpenSession(row.id,'');toast(close?'会话已关闭':'浏览器已打开'); }
 catch(err) { showError(err); }
 finally { busy.delete(row.id);await refresh();render(); }
}
async function showCreate(type) {
 if(creating)return;kind=type;
 $('create-form').reset();$('create-error').hidden=true;$('custom-browser').hidden=true;
 $('name-field').hidden=type==='temporary';$('session-name').required=type!=='temporary';$('launch-field').hidden=type==='temporary';
 $('create-title').textContent=type==='temporary'?'打开一个临时空间':'创建独立空间';
 $('create-intro').textContent=type==='temporary'?'全新、彼此独立。浏览器完全退出后自动删除。':'登录一次，下次打开继续使用。';
 $('create-submit').textContent=type==='temporary'?'打开临时会话':'创建并打开';
 $('browser-choice').replaceChildren(new Option('自动选择 Chrome / Chromium','auto'),new Option('手动指定浏览器…','custom'));
 $('create-dialog').showModal();
 try { const browsers=await api().Browsers();
  // A cancelled dialog may already have been reopened: choices are harmless,
  // but preserve the user's current selection while detection completes.
  const selected=$('browser-choice').value;
  const options=browsers.map(b=>new Option(browserLabel(b.id)+' — '+b.executable,b.executable));
  $('browser-choice').replaceChildren(new Option(browsers.length?'自动选择 Chrome / Chromium':'未检测到浏览器，请手动指定','auto'),...options,new Option('手动指定浏览器…','custom'));
  $('browser-choice').value=selected;
  if(!browsers.length && selected==='auto'){$('browser-choice').value='custom';$('custom-browser').hidden=false;}
 }catch(err){formError('create-error',err);}
}
function showDetail(id) {
 const row=rows.find(r=>r.id===id);if(!row)return;detailID=id;
 $('detail-title').textContent=row.name;$('detail-type').textContent=row.type==='temporary'?'临时空间 · 浏览器完全退出后自动清理':'持久空间 · 下次打开保留登录状态';
 $('details').replaceChildren();
 for(const [label,value] of [['当前状态',statusLabel(row.status)],['浏览器',row.browser.executable],['数据目录',row.dataDir],['创建时间',new Date(row.createdAt).toLocaleString()]]) $('details').append(node('dt','',label),node('dd','',value));
 $('detail-notice').textContent=row.notice;$('detail-notice').hidden=!row.notice;
 $('delete-button').disabled=!canOpen(row) || busy.has(id);
 $('delete-button').title=canOpen(row)?'':'请先关闭浏览器；状态未知时不允许删除';
 $('detail-dialog').showModal();
}
async function cleanup(manual=false) {
 $('cleanup-button').disabled=true;
 try { const notes=await api().Cleanup();$('notice-banner').textContent=notes?.length?'部分临时会话暂未清理：'+notes.join('；'):'';$('notice-banner').hidden=!notes?.length;if(manual && !notes?.length)toast('清理完成，正在使用的会话已保留');await refresh(); }
 catch(err){showError(err);}finally{$('cleanup-button').disabled=false;}
}
for(const nav of document.querySelectorAll('[data-filter]')) nav.addEventListener('click',()=>{
 filter=nav.dataset.filter;for(const other of document.querySelectorAll('[data-filter]')){const selected=other===nav;other.classList.toggle('selected',selected);if(selected)other.setAttribute('aria-current','page');else other.removeAttribute('aria-current');}
 $('page-title').textContent=({all:'全部会话',persistent:'持久会话',temporary:'临时会话',running:'正在运行'})[filter];render();
});
$('search').addEventListener('input',event=>{query=event.target.value;render();});
$('refresh-button').addEventListener('click',()=>refresh(true));$('cleanup-button').addEventListener('click',()=>cleanup(true));
$('dismiss-error').addEventListener('click',()=>$('error-banner').hidden=true);
$('new-button').addEventListener('click',()=>showCreate('persistent'));$('empty-create').addEventListener('click',()=>showCreate('persistent'));$('temp-button').addEventListener('click',()=>showCreate('temporary'));
$('about-button').addEventListener('click',()=>$('about-dialog').showModal());
for(const button of document.querySelectorAll('[data-dismiss]'))button.addEventListener('click',()=>$(button.dataset.dismiss).close());
$('browser-choice').addEventListener('change',()=>$('custom-browser').hidden=$('browser-choice').value!=='custom');
$('launch-now').addEventListener('change',()=>$('create-submit').textContent=$('launch-now').checked?'创建并打开':'创建会话');
$('browse-button').addEventListener('click',async()=>{ $('browse-button').disabled=true;try{const path=await api().PickBrowser();if(path)$('browser-path').value=path;}catch(err){formError('create-error',err);}finally{$('browse-button').disabled=false;} });
$('create-form').addEventListener('submit',async event=>{
 event.preventDefault();if(creating)return;
 const choice=$('browser-choice').value==='custom'?$('browser-path').value.trim():$('browser-choice').value;
 if(!choice){formError('create-error','请选择浏览器文件，或填写可执行文件的完整路径。');return;}
 creating=true;$('create-error').hidden=true;
 const controls=[...$('create-form').elements];for(const control of controls)control.disabled=true;
 const oldLabel=$('create-submit').textContent;$('create-submit').textContent='正在创建…';
 try{await api().CreateSession($('session-name').value,kind,choice,$('create-url').value,$('launch-now').checked);$('create-dialog').close();toast(kind==='temporary'?'临时空间已打开':'会话已创建');}
 catch(err){formError('create-error',err);}
 finally{creating=false;for(const control of controls)control.disabled=false;$('create-submit').textContent=oldLabel;await refresh();}
});
$('create-dialog').addEventListener('cancel',event=>{if(creating)event.preventDefault();});
$('delete-button').addEventListener('click',()=>{
 const row=rows.find(r=>r.id===detailID);if(!row)return;deleteID=row.id;$('detail-dialog').close();$('delete-name').textContent=row.name;$('delete-form').reset();$('delete-error').hidden=true;$('delete-submit').disabled=true;$('delete-dialog').showModal();
});
$('delete-confirmation').addEventListener('input',()=>{$('delete-submit').disabled=$('delete-confirmation').value!==$('delete-name').textContent;});
$('delete-form').addEventListener('submit',async event=>{
 event.preventDefault();if(deleting)return;deleting=true;
 const controls=[...$('delete-form').elements];for(const control of controls)control.disabled=true;
 try{await api().DeleteSession(deleteID,$('delete-confirmation').value);$('delete-dialog').close();toast('会话及其数据已删除');}
 catch(err){formError('delete-error',err);}
 finally{deleting=false;for(const control of controls)control.disabled=false;await refresh();}
});
$('delete-dialog').addEventListener('cancel',event=>{if(deleting)event.preventDefault();});
// No overlapping polling, and no background scans while the window is hidden.
async function poll(){if(!document.hidden)await refresh();setTimeout(poll,3000);}
render();
// Wails injects the bridge before DOM ready. Start on the next task so initial
// UI rendering is never blocked by discovery or orphan cleanup.
setTimeout(async()=>{await refresh();if(ready)await cleanup();setTimeout(poll,3000);},0);
