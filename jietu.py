#coding=utf-8
"""
jietu
"""
import os
import datetime
import time
import copy
import argparse
import logging
from logging.handlers import RotatingFileHandler
import multiprocessing
from urllib.parse import urlparse
import yaml
from selenium import webdriver
from selenium.webdriver.chrome.options import Options

APP_DIR_ROOT = os.path.split(os.path.realpath(__file__))[0]
DATA_DIR_ROOT = os.path.join(APP_DIR_ROOT, "data")
LOGS_DIR_ROOT = os.path.join(APP_DIR_ROOT, "logs")

USER_DATA_DIR = os.path.join(APP_DIR_ROOT, "chromeuserdatadir")

CONF_URL_LIST = os.path.join(APP_DIR_ROOT, "config", "urls.yml")

LOGGER = logging.getLogger(__name__)
LOGGER.setLevel(level=logging.INFO)
LOG_FORMATTER = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')

LOG_FILE = os.path.join(LOGS_DIR_ROOT, "log.txt")
FILE_HANDLER = RotatingFileHandler(LOG_FILE, maxBytes=10*1024, backupCount=3)
FILE_HANDLER.setLevel(logging.INFO)
FILE_HANDLER.setFormatter(LOG_FORMATTER)

CONSOLE_HANDLER = logging.StreamHandler()
CONSOLE_HANDLER.setLevel(logging.INFO)
CONSOLE_HANDLER.setFormatter(LOG_FORMATTER)

LOGGER.addHandler(FILE_HANDLER)
LOGGER.addHandler(CONSOLE_HANDLER)

CHROME_OPTIONS = Options()
CHROME_OPTIONS.add_argument("--headless")
CHROME_OPTIONS.add_argument("--disable-gpu")
CHROME_OPTIONS.add_argument("--user-data-dir=" + USER_DATA_DIR)
CHROME_OPTIONS.add_argument('--user-agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/65.0.3325.181 Safari/537.36"')
CHROME_OPTIONS.add_argument("--disable-gpu")
CHROME_OPTIONS.add_argument('--ignore-certificate-errors')

CHROME_OPTIONS.binary_location = "/usr/bin/google-chrome-stable"

def get_snapshot_by_url(url, file_name, width=1920, height=5400):
    """
    get_snapshot_by_url
    """
    if not url or not file_name:
        LOGGER.error('URL or file_name could not be empty.')
        return None

    LOGGER.info("=====START(%s)=====", url)
    LOGGER.info("URL: %s", url)
    ts_now = datetime.datetime.now()
    ts_suffix = "_".join([str(ts_now.weekday()), str(ts_now.hour)])

    window_size = "".join(["--window-size=", str(width), ",", str(height)])
    ch_options = copy.deepcopy(CHROME_OPTIONS)
    ch_options.add_argument(window_size)

    domain_name = None
    domain_name = urlparse(url).netloc
    if not domain_name.index('.'):
        LOGGER.error("cannot parse the URL's hostname.")
        return None

    domain_dir = None
    domain_dir = os.path.join(DATA_DIR_ROOT, domain_name)
    if domain_dir and (not os.path.exists(domain_dir)):
        os.mkdir(domain_dir)
        LOGGER.info('mkdir: %s', domain_dir)

    if not os.path.exists(domain_dir):
        LOGGER.error('%s does not exist.', domain_dir)
        return None

    if USER_DATA_DIR and (not os.path.exists(USER_DATA_DIR)):
        os.mkdir(USER_DATA_DIR)
        LOGGER.info('mkdir: %s', USER_DATA_DIR)

    if not os.path.exists(USER_DATA_DIR):
        LOGGER.error('%s does not exist.', USER_DATA_DIR)
        return None

    full_name = "".join([domain_dir, os.sep, file_name, "_", ts_suffix, ".png"])

    res_return = False
    driver = webdriver.Chrome(options=ch_options)
    try:
        driver.get(url)
        driver.implicitly_wait(3)
        driver.save_screenshot(full_name)
        if os.path.exists(full_name):
            res_return = True
    except Exception as err:
        LOGGER.error(err)
        res_return = False
    finally:
        LOGGER.info("=====END(%s)=====", url)
        driver.quit()

    return res_return


def get_urls_from_yaml():
    """
    get_urls_from_yaml
    """
    if not os.path.exists(CONF_URL_LIST):
        LOGGER.error('CONF_URL_LIST does not exist: %s', CONF_URL_LIST)
        return None
    f_h = open(CONF_URL_LIST, 'r', encoding='utf-8')
    content = f_h.read()
    f_h.close()
    urls = yaml.safe_load(content)
    if urls is None:
        LOGGER.error("cannot parse the file: %s ", CONF_URL_LIST)
        return None

    urls_return = []

    for url in urls:
        if 'url' not in url.keys() or 'file' not in url.keys() or len(url['url']) < 1 or len(url['file']) < 1:
            LOGGER.error("url or file cannot be empty, will skip this line.")
            continue

        win_width = 1920
        win_height = 6480

        if 'width' in url.keys() and url['width'] > 0:
            win_width = url['width']
        if 'height' in url.keys() and url['height'] > 0:
            win_height = url['height']

        url_one = {'url':url['url'], 'file':url['file'], 'width': win_width, 'height': win_height}
        urls_return.append(url_one)

    return urls_return


def run_task(url, file, width, height):
    """
    run_task
    """
    LOGGER.info('pid %d is running, parent id is %d, Task %s', os.getpid(), os.getppid(), url)
    get_snapshot_by_url(url, file, width, height)


def main():
    """
    main
    """
    t_start = time.time()
    argp = argparse.ArgumentParser(description='support --model.')
    argp.add_argument('--model', default='production', help='debug / production')
    args = argp.parse_args()

    urls = get_urls_from_yaml()
    tasks_result = []

    if args.model.lower() == 'debug':
        LOGGER.info("running model: %s .", args.model.lower())
        urls = urls[0:2]

    if not urls is None:
        process_pool = multiprocessing.Pool(processes=4)

        for url in urls:
            tasks_result.append(process_pool.apply_async(run_task, (url['url'], url['file'], url['width'], url['height'])))

        process_pool.close()
        process_pool.join()
        LOGGER.info("all tasks is done.")
        for t_r in tasks_result:
            try:
                tr_get = t_r.get(timeout=3)
            except Exception as err:
                tr_get = "error"
            finally:
                LOGGER.info("TASK RESULT: %s", tr_get)

    LOGGER.info("Elapsed Running Time: %d seconds.", (time.time() - t_start))


if __name__ == '__main__':
    main()
