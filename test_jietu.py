#coding=utf-8
import unittest
import jietu
import argparse
import re

def isInteger(value):
	rec = re.compile(r'[0-9]\d*$')
	if rec.match(value):
		return True
	return False


class jietuTestCase(unittest.TestCase):
	def test_getUrlsFromYAML(self):
		url_keys_required = set(['url','file','width','height'])
		url_key_valid = 1
		width_int_valid = 0
		height_int_valid = 0
		urls = jietu.getUrlsFromYAML()
		for url in urls:
			url_keys = set(url.keys())
			if not url_keys_required.issubset(url_keys):
				url_key_valid = 0
			if 'width' in url_keys and isInteger(str(url['width'])) and url['width'] > 0:
				width_int_valid = 1
			if 'height' in url_keys and isInteger(str(url['height'])) and url['height'] > 0:
				height_int_valid = 1

		self.assertEqual(url_key_valid, 1,'YAML configuration file cannot be parsed correctly.')
		self.assertEqual(width_int_valid, 1,'width should be integer and > 0.')
		self.assertEqual(height_int_valid, 1,'height should be integer and > 0.')

	def test_getSnapShotByUrl(self):
		#pass
		#ignore because the travis cannot have chrome and chromedriver
		isBingImageCaptured = jietu.getSnapShotByUrl(url="https://cn.bing.com/", fileName="https_cn_bing_com", width=1920, height=1080)
		self.assertEqual(isBingImageCaptured, True,'cannot take the screenshot.')





if __name__ == '__main__':
	argp = argparse.ArgumentParser(description='support --model.')
	argp.add_argument('--model', default='production', help='debug / production')
	args = argp.parse_args()
	print(args.model)
	#unittest.main()
	#print(isInteger(43))