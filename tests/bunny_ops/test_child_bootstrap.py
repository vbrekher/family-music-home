from contextlib import closing
import json,pathlib,sqlite3,tempfile,unittest,sys
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parents[2]/'integrations'/'bunny'))
import runtime
class ChildBootstrapTest(unittest.TestCase):
 def test_only_explicit_protected_children_extend_existing_adults(self):
  with tempfile.TemporaryDirectory() as folder:
   path=pathlib.Path(folder)/'family.db'
   with closing(sqlite3.connect(path)) as db,db:
    db.execute('create table user(id text primary key,user_name text,is_admin integer)');db.executemany('insert into user values(?,?,?)',[('parent','admin',1),('member','member',0)])
    db.execute('create table family_child_profile(user_id text,parent_id text,body text)')
   runtime.validate_family_database(path)
   with closing(sqlite3.connect(path)) as db,db:db.execute('insert into user values(?,?,?)',('synthetic-child','Synthetic child',0))
   with self.assertRaises(ValueError):runtime.validate_family_database(path)
   p={'interface':'kids','density':'large','enabled':True,'allowFamilyMessages':False,'storyLibraries':[]}
   with closing(sqlite3.connect(path)) as db,db:db.execute('insert into family_child_profile values(?,?,?)',('synthetic-child','parent',json.dumps(p)))
   runtime.validate_family_database(path)
   p['interface']='full'
   with closing(sqlite3.connect(path)) as db,db:db.execute('update family_child_profile set body=?',(json.dumps(p),))
   runtime.validate_family_database(path)
   with closing(sqlite3.connect(path)) as db,db:db.execute('update user set is_admin=1 where id=?',('synthetic-child',))
   with self.assertRaises(ValueError):runtime.validate_family_database(path)
 def test_existing_member_cannot_be_silently_reclassified(self):
  with tempfile.TemporaryDirectory() as folder:
   path=pathlib.Path(folder)/'family.db'
   with closing(sqlite3.connect(path)) as db,db:
    db.execute('create table user(id text,user_name text,is_admin integer)');db.executemany('insert into user values(?,?,?)',[('parent','admin',1),('member','member',0)]);db.execute('create table family_child_profile(user_id text,parent_id text,body text)');db.execute('insert into family_child_profile values(?,?,?)',('member','parent','{}'))
   with self.assertRaises(ValueError):runtime.validate_family_database(path)
