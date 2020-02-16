#! /bin/env python3
import os
import time
import requests
from selenium import webdriver
from multiprocessing import Pool
import multiprocessing as mp


DOWNLOAD_ROOT_DIR = "/Volumes/HDD3/bot"
FILE_LIST="/".join([DOWNLOAD_ROOT_DIR,"filelist.txt"])
FILE_LIST_NOT_DOWNLOADED="/".join([DOWNLOAD_ROOT_DIR,"file_not_downloaded.txt"])

ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.1 Safari/603.1.15"
req_headers = {"user-agent": ua}

urls = []
#urls.append("https://www.192td.com/gc/rosimm/rosi2942_#.html")
#urls.append("https://www.192td.com/gq/meinv/xiaoyu192_#.html")

for i in range(195,211):
	url = "".join(["https://www.192td.com/gq/meinv/xiaoyu",str(i),"_#.html"])
	print(url)
	urls.append(url)




def download_images_from_urls(url_tmp=None):
	if url_tmp==None:
		print("url could not be null.")
		return

	global DOWNLOAD_ROOT_DIR, FILE_LIST, FILE_LIST_NOT_DOWNLOADED, ua, req_headers

	url_dir = url_tmp[(url_tmp.rfind("/")+1):(url_tmp.rfind("_#"))]
	img_dir = os.path.join(DOWNLOAD_ROOT_DIR,"data",url_dir)
	print(img_dir)
	if os.path.exists(img_dir):
		print(img_dir+ " exists, will be ignored.")
		return

	

	options = webdriver.ChromeOptions()
	options.add_argument('--headless')
	options.add_argument('--disable-gpu')
	options.add_argument('user-agent=%s'%ua)
	prefs = {"profile.managed_default_content_settings.images": 2}
	options.add_experimental_option("prefs", prefs)

	max_num = 0
	driver = webdriver.Chrome('/usr/local/bin/chromedriver',options = options)  # Optional argument, if not specified will search path.

	for i in range(1,111):
		print("======="+str(i)+"========")
		if i == 1:
			url = url_tmp.replace("_#","")
		else:
			url = url_tmp.replace("#",str(i))
		print(url)
		if max_num > 2:
			print("max_num"+str(max_num))
			break
		

		driver.get(url);
		time.sleep(0.3)
		#time.sleep(1) # Let the user actually see something!
		try:
			img_src = driver.find_element_by_xpath('//*[@id="p"]/center/img').get_attribute("src") 
			if img_src != None:
				img_src=img_src.strip()
				print(img_src)
				img_name=img_src[img_src.rfind('/')+1:]
				img_ext=img_src[img_src.rfind('.'):]
				img_name=str(i)+img_ext
				img_path="/".join([img_dir,img_name])
				print(img_path)
				if os.path.exists(img_dir) == False:
					os.makedirs(img_dir, exist_ok=False)

				if os.path.exists(img_path):
					continue

				line="".join([img_path,"|",img_src,"\n"])

				with open(FILE_LIST, 'a') as f:
					f.write(line)
		except:
			print("Errrrror")
			max_num+=1
		else:
			print("OOOOOOK")

	driver.close()

def download_images_from_filelist():
	global DOWNLOAD_ROOT_DIR, FILE_LIST, FILE_LIST_NOT_DOWNLOADED, ua, req_headers

	flist=open(FILE_LIST,'r')
	lines=flist.readlines()
	flist.close()
	n=0

	for line in lines:
		print("======"+str(n)+"=======")
		arr_line=line.split("|")
		img_src=arr_line[1].strip()
		img_path=arr_line[0].strip()
		img_dir=img_path[:(img_path.rfind("/"))]
		
		print(img_dir+" : "+img_path+" : "+img_src)
		try:
			if os.path.exists(img_dir) == False:
				os.makedirs(img_dir, exist_ok=False)

			if os.path.exists(img_path) and os.path.getsize(img_path) > 10 :
				print(img_path + " exists, will be ignored.")
				continue
			
			res = requests.get(img_src,headers=req_headers, stream=True, timeout=10)
			#print(res.content)
			print(res.status_code) 
			if res.status_code == 200:
				with open(img_path, 'wb') as f:
					f.write(res.content)
					time.sleep(1)
		except:
			with open(FILE_LIST_NOT_DOWNLOADED, 'a') as f:
				f.write("".join([img_path,"|",img_src,"\n"]))
			
			print("Errrrror: " + img_path)
		else:
			print("OOOOOOK")
		n+=1	






def job(img_path_src=None):
	img_path_src=img_path_src.strip()
	if img_path_src == None:
		return
	global req_headers
	arr_path_src = img_path_src.split("|")
	if len(arr_path_src) == 2:
		img_path=arr_path_src[0]
		img_src=arr_path_src[1]
		img_dir=img_path[:(img_path.rfind("/"))]
		if os.path.exists(img_path) and os.path.getsize(img_path) > 10 :
			print(img_path + " exists, will be ignored.")
			return
		if os.path.exists(img_dir) == False:
			os.makedirs(img_dir, exist_ok=False)
		try:
			time.sleep(0.1)
			res = requests.get(img_src,headers=req_headers, stream=True, timeout=10)
			print(str(res.status_code) + ":" + img_path) 
			if res.status_code == 200:
				with open(img_path, 'wb') as f:
					f.write(res.content)
					
		except Exception as e:
			print("Errooooooor: " + img_path + ":" + img_src)
		else:
			print("Oook: " + img_path)
		finally:
			pass

#if os.path.exists("filelist.txt"):
#	os.remove("filelist.txt")	

for url in urls:
	download_images_from_urls(url)
	
#time.sleep(1)
#download_images_from_filelist()


flist=open(FILE_LIST,'r')
lines=flist.readlines()
flist.close()

arr_img_path_src=[]

for line in lines:
	arr_img_path_src.append(line.strip())


t_start=time.time()
with Pool(processes=4) as pool:
	pool.map(job,arr_img_path_src)

print("Elapse: "+str(time.time() - t_start))

