WITH professions(id,kind,starter) AS (
 VALUES ('swordsman','剑','剑-1'),('blade','刀','刀-1'),('boxer','拳套','拳套-1')
)
UPDATE wuxia_characters c
SET inventory = c.inventory || jsonb_build_object(p.starter,1)
FROM professions p
WHERE c.profession_id=p.id AND NOT EXISTS (
 SELECT 1 FROM jsonb_each_text(c.inventory) i
 WHERE i.key=p.starter AND i.value::bigint>0
) AND NOT EXISTS (
 SELECT 1 FROM jsonb_each_text(c.inventory) i
 WHERE i.key IN (p.kind||'-2',p.kind||'-3') AND i.value::bigint>0
 AND c.experience >= CASE WHEN i.key=p.kind||'-2' THEN 360 ELSE 2160 END
);

WITH professions(id,kind,starter) AS (
 VALUES ('swordsman','剑','剑-1'),('blade','刀','刀-1'),('boxer','拳套','拳套-1')
), selected AS (
 SELECT c.group_id,c.user_id,p.kind,
 CASE
 WHEN c.experience>=2160 AND COALESCE((c.inventory->>(p.kind||'-3'))::bigint,0)>0 THEN p.kind||'-3'
 WHEN c.experience>=360 AND COALESCE((c.inventory->>(p.kind||'-2'))::bigint,0)>0 THEN p.kind||'-2'
 ELSE p.starter END AS weapon
 FROM wuxia_characters c JOIN professions p ON c.profession_id=p.id
)
UPDATE wuxia_characters c
SET weapon_kind=s.kind, weapons=c.weapons||jsonb_build_object(s.kind,s.weapon)
FROM selected s WHERE c.group_id=s.group_id AND c.user_id=s.user_id;
