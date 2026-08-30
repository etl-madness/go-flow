<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="text" encoding="UTF-8" media-type="text/csv" />

    <xsl:template match="/pipeline">
        <!-- Visio Data Visualizer Required Header -->
        <xsl:text>Process Step ID,Process Step Description,Next Step ID,Shape Type,Connector Label&#10;</xsl:text>
        
        <!-- Start Node -->
        <xsl:text>Start_Node,Start Pipeline Execution,</xsl:text>
        <xsl:apply-templates select="(*[self::preflight or self::flow]/*)[1]" mode="get-id"/>
        <xsl:text>,Start / End,&#10;</xsl:text>
        
        <!-- Process Pipeline Nodes -->
        <xsl:apply-templates select="//script | //sql | //sql-bulk | //sql_bulk | //http-client | //http_client | //*[local-name()='if'] | //parallel | //foreach | //loop | //while" />

        <!-- End Node -->
        <xsl:text>End_Node,End Pipeline Execution,,Start / End,&#10;</xsl:text>
    </xsl:template>

    <xsl:template match="script | sql | sql-bulk | sql_bulk | http-client | http_client | *[local-name()='if'] | parallel | foreach | loop | while">
        <xsl:variable name="id"><xsl:apply-templates select="." mode="get-id"/></xsl:variable>
        <xsl:variable name="nextId">
            <xsl:choose>
                <xsl:when test="following-sibling::*">
                    <xsl:apply-templates select="following-sibling::*[1]" mode="get-id"/>
                </xsl:when>
                <xsl:otherwise>End_Node</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>
        <xsl:variable name="shapeType">
            <xsl:choose>
                <xsl:when test="local-name()='if'">Decision</xsl:when>
                <xsl:when test="local-name()='parallel'">Subprocess</xsl:when>
                <xsl:otherwise>Process</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>

        <!-- CSV Row Output -->
        <xsl:value-of select="$id"/><xsl:text>,"</xsl:text>
        <xsl:value-of select="$id"/><xsl:text> (</xsl:text><xsl:value-of select="name()"/><xsl:text>)",</xsl:text>
        <xsl:value-of select="$nextId"/><xsl:text>,</xsl:text>
        <xsl:value-of select="$shapeType"/><xsl:text>,&#10;</xsl:text>
    </xsl:template>

    <!-- ID Resolver -->
    <xsl:template match="*" mode="get-id">
        <xsl:choose>
            <xsl:when test="@id and normalize-space(@id) != ''"><xsl:value-of select="@id"/></xsl:when>
            <xsl:otherwise><xsl:value-of select="name()"/>_<xsl:value-of select="generate-id()"/></xsl:otherwise>
        </xsl:choose>
    </xsl:template>
</xsl:stylesheet>