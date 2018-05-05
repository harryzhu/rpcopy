#coding=utf-8
"""
jietu unittest
"""
import unittest
import re
import jietu

def is_integer(value):
    """
    check if the param value is integer and > 0
    """
    rec = re.compile(r'[0-9]\d*$')
    return rec.match(value)


class JietuTestCase(unittest.TestCase):
    """
    test case
    """
    def test_get_urls_from_yaml(self):
        """
        parse the yaml config
        """
        url_keys_required = set(['url', 'file', 'width', 'height'])
        url_key_valid = 0
        width_int_valid = 0
        height_int_valid = 0
        urls = jietu.get_urls_from_yaml()
        for url in urls:
            url_keys = set(url.keys())
            if url_keys_required == url_keys:
                url_key_valid = 1
            if 'width' in url_keys and is_integer(str(url['width'])) and url['width'] > 0:
                width_int_valid = 1
            if 'height' in url_keys and is_integer(str(url['height'])) and url['height'] > 0:
                height_int_valid = 1

        self.assertEqual(url_key_valid, 1, 'YAML configuration file cannot be parsed correctly.')
        self.assertEqual(width_int_valid, 1, 'width should be integer and > 0.')
        self.assertEqual(height_int_valid, 1, 'height should be integer and > 0.')

    def test_get_snap_shot_by_url(self):
        """
        try to take one snapshot by url
        """
        #pass
        #ignore because the travis cannot have chrome and chromedriver
        is_bing_image_captured = jietu.get_snapshot_by_url(url="https://cn.bing.com/", file_name="https_cn_bing_com", width=1920, height=1080)
        self.assertEqual(is_bing_image_captured, True, 'cannot take the screenshot.')





if __name__ == '__main__':
    unittest.main()
