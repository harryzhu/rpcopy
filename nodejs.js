var http=require('http');
server=http.createServer(function(req,res){
res.writeHead(200,{'Content-Type':'text/plain'});
res.end('Hello World');
})
server.listen(8000);