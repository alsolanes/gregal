// Local visual QA server. No providers, credentials or user data are used.
const http=require('node:http'),fs=require('node:fs'),path=require('node:path');
const root=path.resolve(__dirname,'../internal/web');let active=null,aborted=0;
const send=(event,data)=>active?.write('event: '+event+'\ndata: '+JSON.stringify(data)+'\n\n');
http.createServer(async(req,res)=>{
 const url=new URL(req.url,'http://localhost');
 if(url.pathname==='/api/winds/rotllana/stream'||url.pathname==='/api/winds/chat/stream'){
  req.resume();res.writeHead(200,{'Content-Type':'text/event-stream','Cache-Control':'no-cache'});active=res;
  send('phase',{phase:'speaker:marinada'});send('phase',{phase:'thinking'});
  res.on('close',()=>{if(!res.writableEnded)aborted++;if(active===res)active=null;});return;
 }
 if(url.pathname.startsWith('/qa/')){
  let raw='';for await(const chunk of req)raw+=chunk;const body=raw?JSON.parse(raw):{};
  if(url.pathname==='/qa/phase')send('phase',{phase:body.phase});
  if(url.pathname==='/qa/token')send('token',{token:body.token});
  if(url.pathname==='/qa/finish'){send('done',{history:[{from:'user',text:body.prompt},{from:'pet',speaker_id:'marinada',speaker_name:'Marinada',text:'Ja tinc els criteris: bateria, pes, rendiment i pressupost.'}]});active?.end();}
  res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify({aborted,active:!!active}));return;
 }
 const target=path.resolve(root,'.'+(url.pathname==='/'?'/index.html':decodeURIComponent(url.pathname)));
 if(!target.startsWith(root+path.sep)){res.writeHead(403);res.end();return;}
 fs.readFile(target,(err,data)=>{if(err){res.writeHead(404);res.end();return;}res.setHeader('Content-Type',target.endsWith('.js')?'text/javascript':target.endsWith('.css')?'text/css':'text/html');res.end(data);});
}).listen(8766,'127.0.0.1',()=>process.stdout.write('Winds visual QA: http://127.0.0.1:8766\n'));
