import xml.etree.ElementTree as ET
import sys
import re

content = open(sys.argv[1]).read()
# Remove namespaces so ET doesn't complain
content = re.sub(r'\sxmlns="[^"]+"', '', content, count=1)
root = ET.fromstring(content)

print(f"--- {sys.argv[1]} ---")
for elem in root.findall('.//rect'):
    cl = elem.attrib.get('class', '')
    if 'outer' in cl or 'er' in cl or 'entity' in cl or not cl:
        print(f"RECT: x={elem.attrib.get('x')} y={elem.attrib.get('y')} w={elem.attrib.get('width')} h={elem.attrib.get('height')} class={cl}")

for elem in root.findall('.//text'):
    txt = ''.join(elem.itertext()).strip()
    if txt:
        print(f"TEXT: x={elem.attrib.get('x', '0')} y={elem.attrib.get('y', '0')} val='{txt}'")
