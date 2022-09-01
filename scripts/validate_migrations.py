#!/usr/bin/python3

import argparse
import os
import sys

def validate(migrations_dir: str) -> str:
  unique_ids, num_migrations = set(), 0
  for filename in os.listdir(migrations_dir):
    if not os.path.isfile(f'{migrations_dir}/{filename}') or not filename.endswith('.sql'):
      continue
    if not filename.endswith('.up.sql') and not filename.endswith('.down.sql'):
      sys.exit(f'migration does not follow up-down pattern: {filename}')
    
    id = int(filename.split('_')[0])
    unique_ids.add(int(id))

    num_migrations += 1
  
  if len(unique_ids) != num_migrations:
    sys.exit('migration ids are not unique')
  if max(unique_ids) != num_migrations-1:
    sys.exit('migrations are not strictly increasing')

if __name__ == '__main__':
  parser = argparse.ArgumentParser(description='Validate SQL migrations are serial.')
  parser.add_argument('--migrations', type=str, help='The directory in which to validate migration filenames.')

  args = parser.parse_args()

  validate(args.migrations)
