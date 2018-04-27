FROM neo4j:3.3.3

RUN wget -O /var/lib/neo4j/plugins/apoc-3.3.0.2-all.jar https://github.com/neo4j-contrib/neo4j-apoc-procedures/releases/download/3.3.0.2/apoc-3.3.0.2-all.jar

EXPOSE 7474 7473 7687

CMD ["neo4j"]
