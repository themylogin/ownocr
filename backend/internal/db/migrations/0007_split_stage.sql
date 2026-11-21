-- The Split action used to land its copy in the `done` stage, which named both
-- the output of the action and the end of the pipeline. Split now has a stage of
-- its own. Only the Split action ever made a book carrying `done`, so every one
-- of them belongs in `split`.
UPDATE book SET stage = 'split' WHERE stage = 'done';
