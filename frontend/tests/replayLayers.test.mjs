import test from 'node:test';
import assert from 'node:assert/strict';
import { drawReplayLayers } from '../src/api/replayLayers.ts';

function canvas() {
  const calls = [];
  const ctx = Object.fromEntries(['save','restore','beginPath','stroke','setLineDash','fillRect','strokeRect','ellipse'].map(name =>
    [name, (...args) => calls.push([name,...args])]));
  return {ctx,calls};
}
const frame = {tick: 1,zone_radius: 300, units:[{alive:true,x:100,y:200,vision:150},{alive:false,x:200,y:300,vision:150}]};

test('recorded walls and vision use arena scaling and effective rules', () => {
  const {ctx,calls} = canvas();
  drawReplayLayers(ctx,{arena:{width:1200,height:750},walls:[{x:30,y:60,w:90,h:120}],
    config:{match_rules:{zone:true,show_vision:false}},effective_config:{match_rules:{zone:false,show_vision:true}}},frame,600,375);
  assert.deepEqual(calls.filter(c=>c[0]==='fillRect'),[['fillRect',15,30,45,60]]);
  assert.deepEqual(calls.filter(c=>c[0]==='ellipse'),[['ellipse',50,100,75,75,0,0,Math.PI*2]]);
});

test('show_vision false suppresses all vision circles, zone false suppresses storm', () => {
  const {ctx,calls} = canvas();
  drawReplayLayers(ctx,{config:{match_rules:{zone:false,show_vision:false}}},frame,600,375);
  assert.equal(calls.filter(c=>c[0]==='ellipse').length,0);
});
