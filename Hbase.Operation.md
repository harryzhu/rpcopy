**HBase Operation**

### 创建表:
1)预分配16个region,按HexStringSplit拆分,均衡负载; 2)表名images,列簇名cf

    ./hbase org.apache.hadoop.hbase.util.RegionSplitter -c 16 -f cf images HexStringSplit

###  HBase自带的压力测试

    ./hbase org.apache.hadoop.hbase.PerformanceEvaluation sequentialWrite 1
    ./hbase org.apache.hadoop.hbase.PerformanceEvaluation sequentialRead 1
    ./hbase org.apache.hadoop.hbase.PerformanceEvaluation randomWrite 1
    ./hbase org.apache.hadoop.hbase.PerformanceEvaluation randomRead 1

###  为表添加 压缩
    alter 'images',{NAME=>'cf',COMPRESSION=>'GZ'}

###  为表添加 默认过滤器
    alter 'images',{NAME=>'cf',BLOOMFILTER=>'ROW'}

###  压缩表
    major_compact 'images'

###  将缓存写入硬盘
    flush 'images'