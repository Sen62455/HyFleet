@{
    Repository   = "Sen62455/HyFleet"
    Architecture = "amd64"
    Nodes        = @(
        @{
            Name       = "DMIT"
            Target     = "root@DMIT_IP"
            Port       = 22
            Components = @("server", "agent")
        }
        @{
            Name       = "BandwagonHost"
            Target     = "root@BANDWAGONHOST_IP"
            Port       = 22
            Components = @("agent")
        }
        @{
            Name       = "LisaHost"
            Target     = "root@LISAHOST_IP"
            Port       = 22
            Components = @("agent")
        }
    )
}
