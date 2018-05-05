#coding=utf-8
import os
import datetime
import time
import yaml
import copy
import argparse
import logging
from logging.handlers import RotatingFileHandler
import multiprocessing
from urllib.parse import urlparse
from selenium import webdriver
from selenium.webdriver.chrome.options import Options

APP_DIR_ROOT = os.path.split(os.path.realpath(__file__))[0]
DATA_DIR_ROOT = os.path.join(APP_DIR_ROOT,"data")
LOGS_DIR_ROOT = os.path.join(APP_DIR_ROOT,"logs")

USER_DATA_DIR= os.path.join(APP_DIR_ROOT,"chromeuserdatadir")

CONF_URL_LIST = os.path.join(APP_DIR_ROOT,"config","urls.yml")

logger = logging.getLogger(__name__)
logger.setLevel(level=logging.INFO)
formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')

logFile = os.path.join(LOGS_DIR_ROOT,"log.txt")
rHandler = RotatingFileHandler(logFile, maxBytes=10*1024, backupCount = 3)
rHandler.setLevel(logging.INFO)
rHandler.setFormatter(formatter)

console = logging.StreamHandler()
console.setLevel(logging.INFO)
console.setFormatter(formatter)

logger.addHandler(rHandler)
logger.addHandler(console)

chrome_options = Options()
chrome_options.add_argument("--headless")
chrome_options.add_argument("--disable-gpu")
chrome_options.add_argument("--user-data-dir=" + USER_DATA_DIR)
chrome_options.add_argument('--user-agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/65.0.3325.181 Safari/537.36"')
chrome_options.add_argument("--disable-gpu")
chrome_options.add_argument('--ignore-certificate-errors')

chrome_options.binary_location="/usr/bin/google-chrome-stable"
#chrome_options.binary_location="google-chrome-stable"

def getSnapShotByUrl(url="",fileName="",width=1920,height=5400):
	if len(url) == 0 or len(fileName) == 0 :
		logger.error('URL or fileName could not be empty.')
		return None

	logger.info("=====START(%s)=====" % url)
	logger.info("URL: %s" % url)
	NOW=datetime.datetime.now()
	ts_suffix = "_".join([str(NOW.weekday()),str(NOW.hour)])

	ws = "".join(["--window-size=",str(width),",",str(height)])
	co = copy.deepcopy(chrome_options)
	co.add_argument(ws)

	domainName = None
	domainName = urlparse(url).netloc
	if not domainName.index('.'):
		logger.error("cannot parse the URL's hostname.")
		return None
		
	domainDir = None
	domainDir = os.path.join(DATA_DIR_ROOT,domainName)
	if (not domainDir is None) and (not os.path.exists(domainDir)):
		os.mkdir(domainDir)
		logger.info('mkdir: %s' % domainDir)

	if not os.path.exists(domainDir):
		logger.error('%s does not exist.' % domainDir)
		return None

	if (not USER_DATA_DIR is None) and (not os.path.exists(USER_DATA_DIR)):
		os.mkdir(USER_DATA_DIR)
		logger.info('mkdir: %s' % USER_DATA_DIR)

	if not os.path.exists(USER_DATA_DIR):
		logger.error('%s does not exist.' % USER_DATA_DIR)
		return None

	fn = "".join([domainDir,os.sep,fileName,"_",ts_suffix,".png"])

	res_return = False
	driver = webdriver.Chrome(options=co)
	try:
		driver.get(url)
		driver.implicitly_wait(5)
		driver.save_screenshot(fn)
		if os.path.exists(fn):
			res_return = True
	except Exception as e:
		logger.error(e)
		res_return = False
	finally:
		logger.info("=====END(%s)=====" % url)
		driver.quit()

	return res_return


def getUrlsFromYAML():
	if not os.path.exists(CONF_URL_LIST):
		logger.error('CONF_URL_LIST does not exist: %s' % CONF_URL_LIST)
		return None
	f = open(CONF_URL_LIST,'r',encoding='utf-8')
	content = f.read()
	f.close()
	urls = yaml.safe_load(content)
	if urls is None:
		logger.error("cannot parse the file: %s " % CONF_URL_LIST)
		return None

	urls_return = []

	for url in urls:
		if 'url' not in url.keys() or 'file' not in url.keys() or len(url['url']) < 1 or len(url['file']) < 1:
			logger.error("url or file cannot be empty, will skip this line.")
			continue

		winWidth = 1920
		winHeight = 6480
		if 'width' in url.keys() and url['width'] > 0:
			winWidth = url['width']
		if 'height' in url.keys() and url['height'] > 0 :
			winHeight = url['height']
		
		u = {'url':url['url'], 'file':url['file'], 'width': winWidth, 'height': winHeight}
		urls_return.append(u)

	return urls_return
		
def run_task(url,file,width,height):
    logger.info('pid {0} is running, parent id is {1}, Task {2}'.format(os.getpid(), os.getppid(), url))
    getSnapShotByUrl(url,file,width,height)

if __name__ == '__main__':
	t_start = time.time()
	argp = argparse.ArgumentParser(description='support --model.')
	argp.add_argument('--model', default='production', help='debug / production')
	args = argp.parse_args()
	
	urls = getUrlsFromYAML()
	if args.model.lower() == 'debug':
		urls = urls[0:1]
	tasks_result = []
	if urls is not None:
		processPool = multiprocessing.Pool(processes = 4)

		for url in urls:
			tasks_result.append(processPool.apply_async(run_task, (url['url'],url['file'],url['width'],url['height'])))
		
		processPool.close()
		processPool.join()
		logger.info("all tasks is done.")
		for tr in tasks_result:
			logger.info("TASK RESULT: %s" % tr.get(timeout=1))

	logger.info("Elapsed Running Time: %d seconds." % (time.time() - t_start))

