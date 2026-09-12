// Test-only in-memory bridge. Never embedded in the released application.
(() => {
 const make=(name,type,status,id)=>({id,name,type,status,browser:{id:'chrome',executable:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'},dataDir:'/test/browser-sessions/'+id+'/browser-data',createdAt:'2026-09-12T08:00:00Z',notice:''});
 let data=[make('personal','persistent','running','a'),make('work','persistent','stopped','b'),make('github-a','persistent','running','c'),make('github-b','persistent','stopped','d'),make('temporary-4e218f','temporary','running','e')];
 let n=0;
 const pause=()=>new Promise(resolve=>setTimeout(resolve,300));
 window.go={desktop:{App:{
  Snapshot:async()=>({rows:structuredClone(data),root:'/test/browser-sessions',version:'GUI preview · synthetic data'}),
  Browsers:async()=>[{id:'chrome',executable:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'}],
  Cleanup:async()=>[],PickBrowser:async()=>'',
  CreateSession:async(name,type,choice,url,launch)=>{await pause();if(data.some(r=>r.name===name))throw Error('session name already exists');if(type==='temporary')name='temporary-preview-'+(++n);const row=make(name,type,launch?'running':'stopped','preview-'+(++n));data.push(row);return row;},
  OpenSession:async(id)=>{await pause();data.find(r=>r.id===id).status='running';},
  CloseSession:async(id)=>{await pause();const row=data.find(r=>r.id===id);if(row.type==='temporary')data=data.filter(r=>r.id!==id);else row.status='stopped';},
  DeleteSession:async(id,name)=>{if(data.find(r=>r.id===id).name!==name)throw Error('confirmation required');data=data.filter(r=>r.id!==id);}
 }}};
})();
