import {test} from 'node:test';
import assert from 'node:assert/strict';
import {requestFor} from './request.js';
test('uses the exact current HTTP URL without reading or copying account state',()=>{
 assert.deepEqual(requestFor({url:'https://example.com/a?q=x#section'}),{op:'new-session',url:'https://example.com/a?q=x#section'});
});
test('rejects internal pages, credentials and missing active-tab grants',()=>{
 for(const url of [undefined,'chrome://extensions','file:///tmp/a','javascript:alert(1)','https://user:pass@example.com','https://example.com/'+ 'x'.repeat(8192)]) assert.throws(()=>requestFor({url}));
});
