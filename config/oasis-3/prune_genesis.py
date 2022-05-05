import json

def top_k(d, n):
  dn = {}
  while n:
    try:
      k, v = d.popitem()
      dn[k] = v
    except:
      break
    n -= 1
  return dn

if __name__ == '__main__':
  in_file = open('genesis.json', 'r')
  g = json.load(in_file)

  g['registry']['entities'] = g['registry']['entities'][:1]
  g['registry']['nodes'] = g['registry']['nodes'][:1]

  g['staking']['ledger'] = top_k(g['staking']['ledger'], 10)
  g['staking']['delegations'] = top_k(g['staking']['delegations'], 10)
  g['staking']['debonding_delegations'] = top_k(g['staking']['debonding_delegations'], 10)

  flagged = []
  for acc in g['staking']['delegations']:
    if acc not in g['staking']['ledger']:
      flagged.append(acc)
  for acc in flagged:
    del g['staking']['delegations'][acc]

  flagged = []
  for acc in g['staking']['debonding_delegations']:
    if acc not in g['staking']['ledger']:
      flagged.append(acc)
  for acc in flagged:
    del g['staking']['debonding_delegations'][acc]

  for proposal in g['governance']['vote_entries']:
    new_entries = []
    for entry in g['governance']['vote_entries'][proposal]:
      voter = entry['voter']
      if voter in g['staking']['ledger']:
        new_entries.append(entry)
    g['governance']['vote_entries'][proposal] = new_entries

  out_file = open('genesis_new.json', 'w')
  json.dump(g, out_file, indent = 2)
