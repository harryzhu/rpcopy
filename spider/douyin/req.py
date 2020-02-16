#! /bin/env python3
import os
import shutil
import time
import random
import urllib
from os import walk
import hashlib
import requests,json
 
path_list = '/Volumes/HDD3/douyin/list/'
path_download =  '/Volumes/HDD3/douyin/v/'
path_done = '/Volumes/HDD3/douyin/done/'

t_unix_now = time.time()
print(t_unix_now)

def save(video_addr,user_dir):
  url_parse=urllib.parse.urlparse(video_addr)
  qsl=urllib.parse.parse_qsl(url_parse.query)
  file_name = ""
  if len(qsl)>0:
    for vid in qsl:
      if vid[0] == "video_id":
        file_name = "v_" + vid[1]
        print("video_id: "+file_name)
        break

  print(";;;;"+file_name)
  if file_name is None or file_name == "":
    file_name = "u_" + hashlib.md5(video_addr.encode(encoding='UTF-8')).hexdigest()
    print("url md5: "+file_name)
  
  file_path = user_dir+"/"+file_name+'.mp4'
  if os.path.exists(file_path):
    print("ignore. the file exists: " +file_path)
    return None
  
  print("save: " +file_path)
  r =requests.get(video_addr)
  if r.status_code == 200:
    file_name = "m_" + hashlib.md5(r.content).hexdigest()
    file_path = user_dir+"/"+file_name+'.mp4'

    if os.path.exists(file_path):
      print("ignore. the file exists: " +file_path)
      return None

    with open (file_path,'wb') as f:
      f.write(r.content)
      #time.sleep(random.randint(1, 4))

def download():
  for (root, dirs, files) in os.walk(path_list):
    for file in files:
      if not file[-4:] == ".txt":
        continue

      fpath = os.path.join(root, file)
      fage=round(t_unix_now - os.path.getmtime(fpath))
      if fage > 36000:
        print("ignore filelist: "+ fpath+": too old. age: "+str(fage)+" seconds.")
        continue
      
      print("start... " + fpath)

      with open(fpath) as f:
        cnt = f.read()
        json_cnt=json.loads(cnt)
        if json_cnt is None:
          continue
        
        aweme_list=json_cnt["aweme_list"]
        if aweme_list is None:
          continue
        
        for line in aweme_list:
          user_uid=line["author"]["nickname"]
          
          if user_uid is None or user_uid == "":
              user_uid=line["author"]["unique_id"]

          if user_uid is None or user_uid == "":
              user_uid=line["author"]["short_id"]

          if user_uid is None or user_uid == "":
              print("Error: user id is None.")
              user_uid="_default"
              #continue
          
          user_dir = path_download+user_uid
          if not os.path.exists(user_dir):
            os.mkdir(user_dir)

          print(user_uid)
          
          try:
            url_list = line["video"]["play_addr"]["url_list"]
          except:
            url_list = None

          if url_list is None or url_list == "":
            continue

          for video_addr in url_list:
            #pass
            save(video_addr,user_dir)
        
        fdone_path = os.path.join(path_done, user_uid,file)
        fdp,fdn=os.path.split(fdone_path)
        if not os.path.exists(fdp):
            os.makedirs(fdp)   

        flist_path = os.path.join(path_list, user_uid,file)
        print(fdone_path)
        shutil.move(flist_path,fdone_path)

download()      
      
    


