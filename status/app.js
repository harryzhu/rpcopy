'use strict';

var restify = require('restify');
var mysql = require('mysql');
var queues = require('mysql-queues');

var DBCONN = mysql.createConnection({
    host: 'mysql.cwrw2uvmjfgn.ap-northeast-1.rds.amazonaws.com',
    user: 'root',
    password: '1QAZ2wsx',
    database: 'data'
});

function handleDisconnect(conn) {
    conn.on('error', function(err) {
        if (!err.fatal) {
            return;
        }

        if (err.code !== 'PROTOCOL_CONNECTION_LOST') {
            throw err;
        }

        console.log('Re-connecting lost connection: ' + err.stack);

        DBCONN = mysql.createConnection({
            host: 'mysql.cwrw2uvmjfgn.ap-northeast-1.rds.amazonaws.com',
            user: 'root',
            password: '1QAZ2wsx',
            database: 'data'
        });
        handleDisconnect(DBCONN);
        DBCONN.connect();
    });
}

handleDisconnect(DBCONN);

queues(DBCONN, false);

function get_options(req, res, next) {
    if (req.params.name && req.params.item) {
        var q = 'select * from status_options where name = "' + req.params.name + '" and item = "' + req.params.item + '" order by time desc limit 1000';
    } else if (req.params.name) {
        var q = 'select * from status_options where name = "' + req.params.name + '" order by item,time desc limit 1000';
    } else {
        var q = 'select distinct(name) from status_options order by name asc limit 1000';
    }

    //console.log(q);
    DBCONN.query(q, function(err, results) {
        if (err) {
            throw err;
        }
        res.send(results);
    });

    return next;
}

function add_options(req, res, next) {
    if (req.params.name && req.params.itemvalue) {
        var r_kv = req.params.itemvalue;

        var arr_r_kv = r_kv.split("&");
        var str_sql = "";
        for (var i = 0; i < arr_r_kv.length; i++) {
            var kv = arr_r_kv[i].split("=");
            if (kv[0]) {
                if (kv[1]) {
                    str_sql += '("' + req.params.name + '","' + kv[0] + '","' + kv[1] + '"),';
                } else {
                    str_sql += '("' + req.params.name + '","' + kv[0] + '","' + '"),';
                }
            }
        }

        str_sql = str_sql.substring(0, str_sql.length - 1);

        var q = "insert into status_options(name,item,value) values " + str_sql;
        //console.log(q);
        console.time("add_options");
        var trans = DBCONN.startTransaction();
        trans.query(q, function(err, results) {
            if (err) {
                throw err;
            }
            console.timeEnd("add_options");
            res.send(results);
        });
        trans.commit(function(err, info) {
            console.log(info);
        });
        trans.execute();
    } else {
        res.send("ERROR:missed params:name,item,value");
    }

    return next;
}

function patch_options(req, res, next) {
    if (req.params.name && req.params.itemvalue) {
        var r_kv = req.params.itemvalue;

        var arr_r_kv = r_kv.split("&");
        arr_r_kv = arr_r_kv.filter(function(n) {
            return n;
        });
        //console.log(arr_r_kv);
        console.time("patch_options");
        var trans = DBCONN.startTransaction();
        for (var i = 0; i < arr_r_kv.length; i++) {
            var kv = arr_r_kv[i].split("=");
            var str_sql = "";
            if (kv[0]) {
                if (kv[1]) {
                    str_sql = 'update status_options set value="' + kv[1] + '" where item="' + kv[0] + '" and name="' + req.params.name + '";';
                    //console.log(str_sql);
                    trans.query(str_sql, function(err, results) {
                        if (err) {
                            throw err;
                            trans.rollback();
                        }
                    });

                } else {
                    str_sql = kv[0] + '="",';
                }
            }
        }

        trans.commit(function(err, info) {
            console.log(info);
        });
        trans.execute();
        console.timeEnd("patch_options");

        res.send("OK");
    } else {
        res.send("ERROR:missed params:name,item,value");
    }

    return next;
}

function add_notification(req, res, next){
    
}


var server = restify.createServer();

server.use(
        function crossOrigin(req, res, next) {
            res.header("Access-Control-Allow-Origin", "*");
            res.header("Access-Control-Allow-Headers", "X-Requested-With");
            return next();
        }
);
/*
 server.use(restify.gzipResponse());
 
 server.use(restify.throttle({
 burst: 100,
 rate: 50,
 ip: true
 }));
 */

server.get('/servers/', get_options);
server.get('/servers/:name', get_options);
server.get('/servers/:name/:item', get_options);

server.post('/servers/:name/:itemvalue', add_options);
server.get('/servers/:name/:itemvalue', add_options);
server.patch('/servers/:name/:itemvalue', patch_options);



server.listen(5000, function() {
    console.log('%s listening at %s', server.name, server.url);
});