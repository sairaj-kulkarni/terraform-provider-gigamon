map_rule_sets = [
  {
    rule_set_id = "2"
    priority    = 2
    aep_id      = 3
    pass_rules = [
      {
        rule_id = 3
        ip_version = {
          ip_version = "v4"
        }
      }
    ]
    drop_rules = [
      {
        rule_id = 4
        ip_version = {
          ip_version = "v6"
        }
      }
    ]
  }
]
