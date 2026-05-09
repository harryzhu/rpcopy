#
USER_DOMAIN="files.rpcopy.com"
#
# 1) change USER_DOMAIN to your own domain
# 2) open openssl.conf, chane the last line: DNS.1 = your own domain (same as USER_DOMAIN)
# 3) run this script in your console
#

openssl req -x509 -newkey rsa:4096 -keyout ca.key -out ca.crt -subj "/CN=${USER_DOMAIN}" -days 3650  -nodes

mkdir server
openssl req -newkey rsa:2048 -nodes -keyout server/server.key -out server/server.csr -subj "/CN={USER_DOMAIN}" -config openssl.conf 

openssl x509 -req -in server/server.csr -out server/server.crt -CA ca.crt -CAkey ca.key -CAcreateserial -days 3650 -extensions v3_req -extfile openssl.conf

mkdir client
openssl req -newkey rsa:2048 -nodes -keyout client/client.key -out client/client.csr -subj "/CN=${USER_DOMAIN}" -config openssl.conf

openssl x509 -req -in client/client.csr -out client/client.crt -CA ca.crt -CAkey ca.key -CAcreateserial -days 3650 -extensions v3_req -extfile openssl.conf

#
# 4) copy ca.crt into cert/ca.crt
# 5) copy server/server.crt into cert/server/server.crt ; copy server/server.key into cert/server/server.key ;
# 6) copy client/client.crt into cert/client/client.crt ; copy client/client.key into cert/client/client.key ;
# all done.
#
#
mkdir ../server
mkdir ../client
#
cp ca.crt ../
#
cp server/server.crt ../server/
cp server/server.key ../server/
#
cp client/client.crt ../client/
cp client/client.key ../client/
#
#
