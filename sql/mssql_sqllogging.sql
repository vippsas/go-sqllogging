-- This file is copied from https://github.com/vippsas/go-sqllogging
-- START EDITS
--
-- END

-- This function will quote values for use with sqllogging. 3 cases:
-- Basic integer types: Ints
-- Null: Empty string
-- Anything else: quotename() of the value converted to varchar, i.e., '[<..>]'
create function [code].log_quote_value(@x sql_variant)
    returns varchar(max) as
begin
    declare @result varchar(max)
    if sql_variant_property(@x, 'BaseType') in ('tinyint', 'smallint', 'int', 'bigint')
        begin
            set @result = convert(varchar(max), convert(bigint, @x))
        end
    else
        begin
            set @result = isnull(quotename(convert(varchar(max), @x)), '')
        end
    return @result
end

go

create procedure [code].log(
    @level varchar(max),
    @k1 varchar(max) = null,
    @v1 sql_variant = null,
    @k2 varchar(max) = null,
    @v2 sql_variant = null,
    @k3 varchar(max) = null,
    @v3 sql_variant = null,
    @k4 varchar(max) = null,
    @v4 sql_variant = null,
    @k5 varchar(max) = null,
    @v5 sql_variant = null,
    @k6 varchar(max) = null,
    @v6 sql_variant = null,
    @k7 varchar(max) = null,
    @v7 sql_variant = null,
    @k8 varchar(max) = null,
    @v8 sql_variant = null,
    @k9 varchar(max) = null,
    @v9 sql_variant = null,
    @table varchar(max) = null,
    @msg varchar(max) = null
)
as begin
    declare @m nvarchar(max) = concat(@level, ':')
    if @v1 is not null set @m = concat(@m, @k1, '=', [code].log_quote_value(@v1), ' ')
    if @v2 is not null set @m = concat(@m, @k2, '=', [code].log_quote_value(@v2), ' ')
    if @v3 is not null set @m = concat(@m, @k3, '=', [code].log_quote_value(@v3), ' ')
    if @v4 is not null set @m = concat(@m, @k4, '=', [code].log_quote_value(@v4), ' ')
    if @v5 is not null set @m = concat(@m, @k5, '=', [code].log_quote_value(@v5), ' ')
    if @v6 is not null set @m = concat(@m, @k6, '=', [code].log_quote_value(@v6), ' ')
    if @v7 is not null set @m = concat(@m, @k7, '=', [code].log_quote_value(@v7), ' ')
    if @v8 is not null set @m = concat(@m, @k8, '=', [code].log_quote_value(@v8), ' ')
    if @v9 is not null set @m = concat(@m, @k9, '=', [code].log_quote_value(@v9), ' ')
    if @table is not null set @m = concat(@m, @table, '')
    if @msg is not null set @m = concat(@m, @msg, ' ')
    raiserror (@m, 0, 0) with nowait
end
