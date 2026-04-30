import xml.etree.ElementTree as ET
import sys
tree = ET.parse(sys.argv[1])
root = tree.getroot()
for rect in root.findall('.//{http://www.w3.org/2000/svg}rect'):
    print(rect.attrib.get('x'), rect.attrib.get('y'), rect.attrib.get('width'), rect.attrib.get('height'))
