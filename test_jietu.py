#coding=utf-8
import unittest
import jietu
import re

def isInteger(value):
	rec = re.compile(r'[0-9]\d*$')
	if rec.match(value):
		return True
	else:
		return False


class jietuTestCase(unittest.TestCase):
	def test_getUrlsFromYAML(self):
		url_key_valid = 1
		width_int_valid = 0
		height_int_valid = 0
		urls = jietu.getUrlsFromYAML()
		for url in urls:
			if 'url' not in url.keys() or 'file' not in url.keys() or 'width' not in url.keys() or 'height' not in url.keys():
				url_key_valid = 0
			if 'width' in url.keys() and isInteger(str(url['width'])) and url['width'] > 0:
				width_int_valid = 1
			if 'height' in url.keys() and isInteger(str(url['height'])) and url['height'] > 0:
				height_int_valid = 1

		self.assertEqual(url_key_valid, 1,'YAML configuration file cannot be parsed correctly.')
		self.assertEqual(width_int_valid, 1,'width should be integer and > 0.')
		self.assertEqual(height_int_valid, 1,'height should be integer and > 0.')

	def test_getSnapShotByUrl(self):
		pass
		#ignore because the travis cannot have chrome and chromedriver
		#isBingImageCaptured = jietu.getSnapShotByUrl(url="https://cn.bing.com/", fileName="https_cn_bing_com", width=1920, height=1080)
		#self.assertEqual(isBingImageCaptured, True,'cannot take the screenshot.')





if __name__ == '__main__':
	unittest.main()
	#print(isInteger(43))