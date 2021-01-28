#!/usr/bin/env python3

import os
import sys
from lxml import etree

def main():
    if len(sys.argv) < 2:
        print("Usage: {} filename (protocol)".format(sys.argv[0]))
        sys.exit(1)
    
    protocol = "tcp"
    if len(sys.argv) == 3:
        protocol = sys.argv[2]

    filename = sys.argv[1]
    if not os.path.exists(filename):
        print("Filename {} does not exists!".format(filename))
        sys.exit(1)

    results = list()
    xml = etree.parse(filename)

    for host in xml.xpath("./host"):
        hnames = list()
        for hname in host.xpath("./hostnames/hostname"):
            if len(hname.xpath("./@name")):
                hnames.append(hname.xpath("./@name")[0])
        
        if len(hnames) == 0:
            break
        
        open_ports = list()
        for port in host.xpath("./ports/port"):
            state = port.xpath("./state/@state")
            
            if len(state) and state[0] == "open":                
                portid = port.xpath("./@portid")
                proto  = port.xpath("./@protocol")
                
                if len(portid) and len(proto) and proto[0] == "tcp":
                    portid = portid[0]
                    open_ports.append(portid)
                    
        if len(open_ports):
            for hname in hnames:
                results.append((hname, ",".join(open_ports)))

    for item in sorted(list(set(results)), key=lambda x: x[0]):
        print("{} {}".format(item[0], item[1]))

if __name__ == "__main__":
    main()