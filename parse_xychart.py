import xml.etree.ElementTree as ET
tree = ET.parse('/tmp/ref-xychart.svg')
root = tree.getroot()
for path in root.findall('.//{http://www.w3.org/2000/svg}path'):
    print(path.attrib.get('d'))
