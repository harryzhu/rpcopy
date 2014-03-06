**Hadoop Setup**

源代码编译安装 [Hadoop 2.3.0](http://mirror.bit.edu.cn/apache/hadoop/common/hadoop-2.3.0/hadoop-2.3.0-src.tar.gz)

**安装依赖 :**



### 配置ssh无密码登录
cd ~/.ssh
ssh-keygen -t rsa
cat id_rsa.pub >> authorized_keys 
chmod 600 authorized_keys
ssh localhost


### 配置core-site.xml

```
<script src="https://gist.github.com/HarryZhu/9396370.js"></script>
```