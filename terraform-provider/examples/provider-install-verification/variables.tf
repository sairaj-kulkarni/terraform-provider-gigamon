variable "map_rule_sets" {
  type = list(object({
    rule_set_id = string  # "1" to "5"
    priority    = number  # 1 to 5
    aep_id      = number  # 2 to 63
    pass_rules = list(object({
      rule_id = number
      ip_version = object({
        ip_version = string
      })
    }))
    drop_rules = list(object({
      rule_id = number
      ip_version = object({
        ip_version = string
      })
    }))
  }))
}
