import {test} from 'node:test';
import assert from 'node:assert/strict';
import {canOpen,isActive,visibleRows,statusLabel} from './web/model.js';
const rows=[{name:'personal',type:'persistent',status:'running',browser:{id:'chrome'}},{name:'work',type:'persistent',status:'stopped',browser:{id:'chromium'}},{name:'temp-a',type:'temporary',status:'unmanaged',browser:{id:'custom'}}];
test('unknown, starting and orphaned processes never enable Open',()=>{
 for(const status of ['unknown','starting','unmanaged','running','invalid'])assert.equal(canOpen({status}),false);
 assert.equal(canOpen({status:'stopped'}),true);
 assert.equal(canOpen({status:'stopped (see status)'}),true);
 assert.equal(statusLabel('unexpected'),'状态未知');
});
test('search and category filters compose without hiding active orphans',()=>{
 assert.deepEqual(visibleRows(rows,'persistent',' CHROMIUM '),[rows[1]]);
 assert.deepEqual(visibleRows(rows,'running',''),[rows[0],rows[2]]);
 assert.deepEqual(visibleRows(rows,'temporary','temp'),[rows[2]]);
 assert.deepEqual(visibleRows(rows,'all','<script>'),[]);
 assert.equal(isActive(rows[2]),true);
});
