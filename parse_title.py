import xml.etree.ElementTree as ET
import sys
tree = ET.parse(sys.argv[1])
root = tree.getroot()
for text in root.iter():
    if text.text and "Platform milestones" in text.text:
        print(text.tag, text.attrib)
