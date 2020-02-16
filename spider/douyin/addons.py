#! /bin/env python3
import os
import requests
import json
import codecs
import hashlib
import mitmproxy.http
from mitmproxy import ctx

URL_PREFIXES = ['https://api3-normal-c-hl.amemv.com/aweme/v1/aweme/post/']

DIR_LIST = '/Volumes/HDD3/douyin/list/'

num = 0


class Counter:
    def __init__(self):
        self.num = 0

    def request(self, flow: mitmproxy.http.HTTPFlow):
        self.num = self.num + 1
        ctx.log.info("We've seen %d flows" % self.num)

    def response(self, flow: mitmproxy.http.HTTPFlow):
        global num
        for url in URL_PREFIXES :
            if flow.request.url.startswith(url):
                response = flow.response
                data = response.content
                if data is None:
                    print("Error: response data is None")
                    continue
                quiz = json.loads(data)
                if quiz is None:
                    print("Error: data jsonfy is None.")
                    continue
                #print(quiz['aweme_list'][0])
               
                user_uid=quiz['aweme_list'][0]["author"]["nickname"]

                if user_uid is None or user_uid == "":
                    user_uid=quiz['aweme_list'][0]["author"]["unique_id"]

                if user_uid is None or user_uid == "":
                    user_uid=quiz['aweme_list'][0]["author"]["short_id"]

                if user_uid is None or user_uid == "":
                    print("Error: user id is None.")
                    user_uid="_default"
                    #continue

                user_dir = os.path.join(DIR_LIST,user_uid)
                if not os.path.exists(user_dir):
                    os.mkdir(user_dir)

                quiz['aweme_list']
                filename = os.path.join(user_dir, str(num)+'.txt')
                save(filename,json.dumps(quiz))
                print(filename + '下载完成')

                print(flow.request.url)
                num += 1

def save(filename, contents):
    fh = open(filename, 'w', encoding='utf-8')
    fh.write(contents)
    fh.close()


addons = [
    Counter()
]